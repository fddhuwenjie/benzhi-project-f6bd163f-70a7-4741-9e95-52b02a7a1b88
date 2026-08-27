package application

import (
	"encoding/json"
	"time"

	"benzhi-project-f6bd163f-70a7-4741-9e95-52b02a7a1b88/internal/domain"
	"benzhi-project-f6bd163f-70a7-4741-9e95-52b02a7a1b88/internal/store"
)

func (s *Service) lookup(requestID, fingerprint string) (*Outcome, error) {
	if requestID == "" {
		return nil, nil
	}
	stored, err := s.repo.LookupResult(requestID, fingerprint)
	if err != nil {
		if e, ok := err.(*domain.Error); ok {
			return &Outcome{Error: e, HTTPStatus: statusForCode(e.Code)}, nil
		}
		return nil, err
	}
	if stored == nil {
		return nil, nil
	}
	var outcome Outcome
	if err := json.Unmarshal(stored.Body, &outcome); err != nil {
		return nil, domain.NewError(domain.CodeIntegrity, "持久幂等结果损坏: %v", err)
	}
	outcome.Replayed = true
	outcome.HTTPStatus = stored.HTTPStatus
	return &outcome, nil
}

func (s *Service) persistFailure(requestID, fingerprint string, businessError *domain.Error) (*Outcome, error) {
	outcome := &Outcome{Error: businessError, HTTPStatus: statusForCode(businessError.Code)}
	if requestID == "" || fingerprint == "" {
		return outcome, nil
	}
	stored, err := encodeStored(requestID, fingerprint, outcome.HTTPStatus, outcome, s.now())
	if err != nil {
		return nil, err
	}
	if err := s.repo.SaveResult(*stored); err != nil {
		return nil, err
	}
	return outcome, nil
}

func encodeStored(requestID, fingerprint string, status int, outcome *Outcome, now time.Time) (*store.StoredResult, error) {
	payload, err := json.Marshal(outcome)
	if err != nil {
		return nil, err
	}
	return &store.StoredResult{RequestID: requestID, Fingerprint: fingerprint, HTTPStatus: status, Body: payload, CreatedAt: now.UTC()}, nil
}
