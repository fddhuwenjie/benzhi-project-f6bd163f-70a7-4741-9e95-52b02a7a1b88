package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"benzhi-project-f6bd163f-70a7-4741-9e95-52b02a7a1b88/internal/domain"
)

type snapshotReadError struct {
	caseID string
	err    error
}

func (e *snapshotReadError) Error() string {
	return fmt.Sprintf("读取案件 %s 的快照: %v", e.caseID, e.err)
}

func (e *snapshotReadError) Unwrap() error {
	return e.err
}

func wrapSnapshotRead(caseID string, err error) error {
	if err == nil {
		return nil
	}
	return &snapshotReadError{caseID: caseID, err: err}
}

func caseFilename(id string) (string, error) {
	if id == "" || strings.ContainsAny(id, `/\\`) || id == "." || id == ".." {
		return "", domain.NewError(domain.CodeValidation, "案件 ID 无效")
	}
	return id + ".json", nil
}

func readCase(path string) (*domain.DrillCase, error) {
	payload, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, domain.NewError(domain.CodeNotFound, "案件不存在")
	}
	if err != nil {
		return nil, fmt.Errorf("读取案件快照: %w", err)
	}
	var c domain.DrillCase
	if err := json.Unmarshal(payload, &c); err != nil {
		return nil, domain.NewError(domain.CodeIntegrity, "案件快照损坏: %v", err)
	}
	return &c, nil
}

func writeJSONAtomic(path string, value any) error {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".atomic-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		tmp.Close()
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		return err
	}
	if _, err := tmp.Write(payload); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	ok = true
	if directory, err := os.Open(dir); err == nil {
		_ = directory.Sync()
		_ = directory.Close()
	}
	return nil
}
