package store

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

const maxFrameSize = 1 << 20

func scanEventLog(path string) (Integrity, []EventFrame, error) {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return Integrity{Healthy: true}, nil, nil
	}
	if err != nil {
		return Integrity{ErrorMessage: err.Error()}, nil, err
	}
	defer file.Close()
	state := Integrity{Healthy: true}
	frames := []EventFrame{}
	caseRevisions := map[string]int64{}
	for {
		var length uint32
		err := binary.Read(file, binary.BigEndian, &length)
		if err == io.EOF {
			break
		}
		if err != nil {
			return broken(state, frames, "事件帧长度被截断: %v", err)
		}
		if length == 0 || length > maxFrameSize {
			return broken(state, frames, "事件帧长度 %d 无效", length)
		}
		payload := make([]byte, length)
		if _, err := io.ReadFull(file, payload); err != nil {
			return broken(state, frames, "事件帧正文被截断: %v", err)
		}
		checksum := make([]byte, sha256.Size)
		if _, err := io.ReadFull(file, checksum); err != nil {
			return broken(state, frames, "事件帧校验和被截断: %v", err)
		}
		sum := sha256.Sum256(payload)
		if !equalBytes(sum[:], checksum) {
			return broken(state, frames, "事件帧 %d 校验和不匹配", state.Frames+1)
		}
		var frame EventFrame
		if err := json.Unmarshal(payload, &frame); err != nil {
			return broken(state, frames, "事件帧无法解析: %v", err)
		}
		if frame.Sequence != state.Frames+1 {
			return broken(state, frames, "事件序号不连续: 期望 %d，实际 %d", state.Frames+1, frame.Sequence)
		}
		if frame.PreviousDigest != state.LastDigest {
			return broken(state, frames, "事件帧 %d 前序摘要不匹配", frame.Sequence)
		}
		if frame.CaseID == "" || frame.Type == "" || frame.Revision != caseRevisions[frame.CaseID]+1 {
			return broken(state, frames, "事件帧 %d 的案件修订不连续", frame.Sequence)
		}
		caseRevisions[frame.CaseID] = frame.Revision
		state.Frames = frame.Sequence
		state.LastDigest = hex.EncodeToString(sum[:])
		frames = append(frames, frame)
	}
	return state, frames, nil
}

func broken(state Integrity, frames []EventFrame, format string, args ...any) (Integrity, []EventFrame, error) {
	err := fmt.Errorf(format, args...)
	state.Healthy = false
	state.ErrorMessage = err.Error()
	return state, frames, err
}

func appendFrame(writer io.Writer, frame EventFrame) (string, error) {
	payload, err := json.Marshal(frame)
	if err != nil {
		return "", err
	}
	if len(payload) > maxFrameSize {
		return "", fmt.Errorf("事件帧超过大小上限")
	}
	sum := sha256.Sum256(payload)
	if err := binary.Write(writer, binary.BigEndian, uint32(len(payload))); err != nil {
		return "", err
	}
	if _, err := writer.Write(payload); err != nil {
		return "", err
	}
	if _, err := writer.Write(sum[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(sum[:]), nil
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var different byte
	for i := range a {
		different |= a[i] ^ b[i]
	}
	return different == 0
}
