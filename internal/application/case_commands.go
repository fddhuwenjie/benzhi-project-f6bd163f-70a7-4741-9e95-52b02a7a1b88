package application

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"benzhi-project-f6bd163f-70a7-4741-9e95-52b02a7a1b88/internal/domain"
	"benzhi-project-f6bd163f-70a7-4741-9e95-52b02a7a1b88/internal/store"
)

func (s *Service) CreateCase(command CreateCaseCommand) (*Outcome, error) {
	fingerprint, err := commandFingerprint(command)
	if err != nil {
		return nil, err
	}
	if invalid := validateMeta(command.CommandMeta); invalid != nil {
		return s.persistFailure(command.RequestID, fingerprint, invalid)
	}
	if command.ExpectedRevision != 0 {
		return s.persistFailure(command.RequestID, fingerprint, &domain.Error{Code: domain.CodeConflict, Message: "创建案件的 expected_revision 必须为 0"})
	}
	if replay, err := s.lookup(command.RequestID, fingerprint); replay != nil || err != nil {
		return replay, err
	}
	caseID := command.CaseID
	if caseID == "" {
		sum := sha256.Sum256([]byte(command.RequestID))
		caseID = "case-" + hex.EncodeToString(sum[:8])
	}
	lock := s.caseLock(caseID)
	lock.Lock()
	defer lock.Unlock()
	if replay, err := s.lookup(command.RequestID, fingerprint); replay != nil || err != nil {
		return replay, err
	}
	c, err := domain.NewCase(caseID, command.Title, command.Building, command.CoordinatorID, command.ObserverIDs, s.now())
	if err != nil {
		if e, ok := err.(*domain.Error); ok {
			return s.persistFailure(command.RequestID, fingerprint, e)
		}
		return nil, err
	}
	outcome := &Outcome{Case: c, HTTPStatus: 201}
	stored, err := encodeStored(command.RequestID, fingerprint, 201, outcome, s.now())
	if err != nil {
		return nil, err
	}
	data, _ := json.Marshal(map[string]string{"title": c.Title, "building": c.Building})
	err = s.repo.Commit(store.CommitInput{Case: c, ExpectedRevision: 0, Event: store.Event{Type: "case.created", ActorID: command.ActorID, Data: data}, Result: stored})
	if err != nil {
		if e, ok := err.(*domain.Error); ok {
			return &Outcome{Error: e, HTTPStatus: statusForCode(e.Code)}, nil
		}
		return nil, err
	}
	return outcome, nil
}

func (s *Service) FreezeBaseline(command FreezeBaselineCommand) (*Outcome, error) {
	return s.execute(command.CaseID, "baseline.frozen", command.CommandMeta, command, 200, func(c *domain.DrillCase) error {
		return c.FreezeBaseline(command.Baseline, command.ActorID, s.now())
	})
}

func (s *Service) RecordPreflight(command PreflightCommand) (*Outcome, error) {
	return s.execute(command.CaseID, "preflight.recorded", command.CommandMeta, command, 200, func(c *domain.DrillCase) error {
		if command.ValidUntil.IsZero() {
			return domain.NewError(domain.CodeValidation, "核验有效截止时间不能为空")
		}
		return c.RecordPreflight(command.Item, command.Passed, command.EvidenceSummary, command.ActorID, command.ValidUntil, s.now())
	})
}

func (s *Service) PrecheckBaseline(command BaselinePrecheckCommand) (*BaselinePrecheckResult, error) {
	c, err := s.repo.Load(command.CaseID)
	if err != nil {
		return nil, err
	}
	if c.Revision != command.ExpectedRevision {
		return nil, domain.NewError(domain.CodeConflict, "修订冲突：期望 %d，当前 %d", command.ExpectedRevision, c.Revision)
	}
	if c.Status != domain.StatusDraft {
		return nil, domain.NewError(domain.CodeState, "仅草拟态可以预检情景")
	}
	if command.ActorID != c.CoordinatorID {
		return nil, domain.NewError(domain.CodeForbidden, "仅案件协调员可以预检情景")
	}
	return &BaselinePrecheckResult{CaseID: c.CaseID, Revision: c.Revision, Precheck: domain.NormalizeBaseline(command.Baseline)}, nil
}
