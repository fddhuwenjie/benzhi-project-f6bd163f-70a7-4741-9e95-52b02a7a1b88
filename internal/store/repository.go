package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"benzhi-project-f6bd163f-70a7-4741-9e95-52b02a7a1b88/internal/domain"
)

type Repository struct {
	dir         string
	casesDir    string
	eventsPath  string
	resultsPath string
	mu          sync.RWMutex
	eventFile   *os.File
	integrity   Integrity
	results     map[string]StoredResult
}

func Open(dir string) (*Repository, error) {
	if dir == "" {
		return nil, fmt.Errorf("数据目录不能为空")
	}
	casesDir := filepath.Join(dir, "cases")
	if err := os.MkdirAll(casesDir, 0o700); err != nil {
		return nil, fmt.Errorf("创建数据目录: %w", err)
	}
	r := &Repository{dir: dir, casesDir: casesDir, eventsPath: filepath.Join(dir, "events.log"), resultsPath: filepath.Join(dir, "idempotency.json"), results: map[string]StoredResult{}}
	state, frames, scanErr := scanEventLog(r.eventsPath)
	r.integrity = state
	if err := r.loadResults(); err != nil {
		return nil, err
	}
	if scanErr == nil {
		scanErr = r.verifySnapshots(frames)
		if scanErr != nil {
			r.integrity.Healthy = false
			r.integrity.ErrorMessage = scanErr.Error()
		}
	}
	if scanErr != nil {
		return r, nil
	}
	file, err := os.OpenFile(r.eventsPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("打开事件日志: %w", err)
	}
	r.eventFile = file
	return r, nil
}

func (r *Repository) verifySnapshots(frames []EventFrame) error {
	latest := map[string]int64{}
	for _, frame := range frames {
		latest[frame.CaseID] = frame.Revision
	}
	entries, err := os.ReadDir(r.casesDir)
	if err != nil {
		return fmt.Errorf("读取案件快照目录: %w", err)
	}
	found := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		c, err := readCase(filepath.Join(r.casesDir, entry.Name()))
		if err != nil {
			return err
		}
		if latest[c.CaseID] != c.Revision {
			return domain.NewError(domain.CodeIntegrity, "案件 %s 的快照修订 %d 与事件日志修订 %d 不一致", c.CaseID, c.Revision, latest[c.CaseID])
		}
		found[c.CaseID] = true
	}
	for caseID := range latest {
		if !found[caseID] {
			return domain.NewError(domain.CodeIntegrity, "事件日志中的案件 %s 缺少快照", caseID)
		}
	}
	return nil
}

func (r *Repository) Events(caseID string) ([]EventFrame, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	state, frames, err := scanEventLog(r.eventsPath)
	if err != nil || !state.Healthy {
		return nil, domain.NewError(domain.CodeIntegrity, "无法读取审计事件: %s", state.ErrorMessage)
	}
	result := make([]EventFrame, 0)
	for _, frame := range frames {
		if frame.CaseID == caseID {
			result = append(result, frame)
		}
	}
	if len(result) == 0 {
		return nil, domain.NewError(domain.CodeNotFound, "案件不存在或没有审计事件")
	}
	return result, nil
}

func (r *Repository) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.eventFile != nil {
		return r.eventFile.Close()
	}
	return nil
}

func (r *Repository) Integrity() Integrity {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.integrity
}

func (r *Repository) Load(caseID string) (*domain.DrillCase, error) {
	filename, err := caseFilename(caseID)
	if err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return readCase(filepath.Join(r.casesDir, filename))
}

func (r *Repository) List() ([]*domain.DrillCase, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entries, err := os.ReadDir(r.casesDir)
	if err != nil {
		return nil, err
	}
	cases := make([]*domain.DrillCase, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		c, err := readCase(filepath.Join(r.casesDir, entry.Name()))
		if err != nil {
			return nil, err
		}
		cases = append(cases, c)
	}
	sort.Slice(cases, func(i, j int) bool { return cases[i].CreatedAt.After(cases[j].CreatedAt) })
	return cases, nil
}

func (r *Repository) Commit(input CommitInput) error {
	if input.Case == nil || input.Case.CaseID == "" || input.Event.Type == "" {
		return domain.NewError(domain.CodeValidation, "提交缺少案件或事件")
	}
	filename, err := caseFilename(input.Case.CaseID)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.integrity.Healthy || r.eventFile == nil {
		return domain.NewError(domain.CodeIntegrity, "事件日志完整性异常，存储已停止写入: %s", r.integrity.ErrorMessage)
	}
	path := filepath.Join(r.casesDir, filename)
	currentRevision := int64(0)
	current, loadErr := readCase(path)
	if loadErr == nil {
		currentRevision = current.Revision
	} else if domain.ErrorCode(loadErr) != domain.CodeNotFound {
		return loadErr
	}
	if currentRevision != input.ExpectedRevision {
		return domain.NewError(domain.CodeConflict, "修订冲突：期望 %d，当前 %d", input.ExpectedRevision, currentRevision)
	}
	if currentRevision == 0 {
		input.Case.Revision = 1
	} else {
		input.Case.Revision = currentRevision + 1
	}
	frame := EventFrame{Sequence: r.integrity.Frames + 1, CaseID: input.Case.CaseID, Revision: input.Case.Revision, Type: input.Event.Type, ActorID: input.Event.ActorID, Data: input.Event.Data, OccurredAt: time.Now().UTC(), PreviousDigest: r.integrity.LastDigest}
	if err := writeJSONAtomic(path, input.Case); err != nil {
		return fmt.Errorf("写入案件快照: %w", err)
	}
	digest, err := appendFrame(r.eventFile, frame)
	if err != nil {
		r.integrity.Healthy = false
		r.integrity.ErrorMessage = err.Error()
		return fmt.Errorf("追加事件日志: %w", err)
	}
	r.integrity.Frames = frame.Sequence
	r.integrity.LastDigest = digest
	if input.Result != nil {
		r.results[input.Result.RequestID] = *input.Result
		if err := writeJSONAtomic(r.resultsPath, r.results); err != nil {
			return fmt.Errorf("写入幂等索引: %w", err)
		}
	}
	return nil
}

func (r *Repository) LookupResult(requestID, fingerprint string) (*StoredResult, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result, ok := r.results[requestID]
	if !ok {
		return nil, nil
	}
	if result.Fingerprint != fingerprint {
		return nil, domain.NewError(domain.CodeIdempotency, "request_id 已被不同请求内容使用")
	}
	copy := result
	return &copy, nil
}

func (r *Repository) SaveResult(result StoredResult) error {
	if result.RequestID == "" || result.Fingerprint == "" {
		return domain.NewError(domain.CodeValidation, "幂等结果缺少键或指纹")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.results[result.RequestID]; ok && existing.Fingerprint != result.Fingerprint {
		return domain.NewError(domain.CodeIdempotency, "request_id 已被不同请求内容使用")
	}
	r.results[result.RequestID] = result
	return writeJSONAtomic(r.resultsPath, r.results)
}

func (r *Repository) loadResults() error {
	payload, err := os.ReadFile(r.resultsPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("读取幂等索引: %w", err)
	}
	if err := json.Unmarshal(payload, &r.results); err != nil {
		return domain.NewError(domain.CodeIntegrity, "幂等索引损坏: %v", err)
	}
	return nil
}
