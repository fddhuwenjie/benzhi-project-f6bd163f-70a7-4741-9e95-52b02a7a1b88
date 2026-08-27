package application

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"benzhi-project-f6bd163f-70a7-4741-9e95-52b02a7a1b88/internal/domain"
	"benzhi-project-f6bd163f-70a7-4741-9e95-52b02a7a1b88/internal/store"
)

type clockFunc func() time.Time

type Service struct {
	repo    *store.Repository
	now     clockFunc
	locksMu sync.Mutex
	locks   map[string]*sync.Mutex
}

func NewService(repo *store.Repository) *Service {
	return &Service{repo: repo, now: func() time.Time { return time.Now().UTC() }, locks: map[string]*sync.Mutex{}}
}

func (s *Service) caseLock(caseID string) *sync.Mutex {
	s.locksMu.Lock()
	defer s.locksMu.Unlock()
	lock := s.locks[caseID]
	if lock == nil {
		lock = &sync.Mutex{}
		s.locks[caseID] = lock
	}
	return lock
}

type mutation func(*domain.DrillCase) error

func (s *Service) execute(caseID, eventType string, meta CommandMeta, command any, status int, mutate mutation) (*Outcome, error) {
	fingerprint, err := commandFingerprint(command)
	if err != nil {
		return nil, err
	}
	if invalid := validateMeta(meta); invalid != nil {
		return s.persistFailure(meta.RequestID, fingerprint, invalid)
	}
	if replay, err := s.lookup(meta.RequestID, fingerprint); replay != nil || err != nil {
		return replay, err
	}
	lock := s.caseLock(caseID)
	lock.Lock()
	defer lock.Unlock()
	if replay, err := s.lookup(meta.RequestID, fingerprint); replay != nil || err != nil {
		return replay, err
	}
	c, err := s.repo.Load(caseID)
	if err != nil {
		if e, ok := err.(*domain.Error); ok {
			return s.persistFailure(meta.RequestID, fingerprint, e)
		}
		return nil, err
	}
	if c.Revision != meta.ExpectedRevision {
		return s.persistFailure(meta.RequestID, fingerprint, &domain.Error{Code: domain.CodeConflict, Message: fmt.Sprintf("修订冲突：期望 %d，当前 %d", meta.ExpectedRevision, c.Revision)})
	}
	if err := mutate(c); err != nil {
		if e, ok := err.(*domain.Error); ok {
			return s.persistFailure(meta.RequestID, fingerprint, e)
		}
		return nil, err
	}
	c.Revision = meta.ExpectedRevision + 1
	outcome := &Outcome{Case: c, HTTPStatus: status}
	stored, err := encodeStored(meta.RequestID, fingerprint, status, outcome, s.now())
	if err != nil {
		return nil, err
	}
	eventData, _ := json.Marshal(command)
	err = s.repo.Commit(store.CommitInput{Case: c, ExpectedRevision: meta.ExpectedRevision, Event: store.Event{Type: eventType, ActorID: meta.ActorID, Data: eventData}, Result: stored})
	if err != nil {
		if e, ok := err.(*domain.Error); ok {
			return &Outcome{Error: e, HTTPStatus: statusForCode(e.Code)}, nil
		}
		return nil, err
	}
	return outcome, nil
}

func validateMeta(meta CommandMeta) *domain.Error {
	if strings.TrimSpace(meta.RequestID) == "" || len(meta.RequestID) > 128 {
		return &domain.Error{Code: domain.CodeValidation, Message: "request_id 不能为空且长度不能超过 128"}
	}
	if strings.TrimSpace(meta.ActorID) == "" {
		return &domain.Error{Code: domain.CodeValidation, Message: "actor_id 不能为空"}
	}
	if meta.ExpectedRevision < 0 {
		return &domain.Error{Code: domain.CodeValidation, Message: "expected_revision 不能为负数"}
	}
	return nil
}

func commandFingerprint(command any) (string, error) {
	payload, err := json.Marshal(command)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func statusForCode(code string) int {
	switch code {
	case domain.CodeValidation, domain.CodeState:
		return 422
	case domain.CodeForbidden:
		return 403
	case domain.CodeNotFound:
		return 404
	case domain.CodeConflict, domain.CodeIdempotency:
		return 409
	case domain.CodeIntegrity:
		return 503
	default:
		return 500
	}
}
