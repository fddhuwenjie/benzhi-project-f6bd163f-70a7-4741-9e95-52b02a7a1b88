package store

import (
	"encoding/json"
	"time"

	"benzhi-project-f6bd163f-70a7-4741-9e95-52b02a7a1b88/internal/domain"
)

type Event struct {
	Type    string          `json:"type"`
	ActorID string          `json:"actor_id"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type EventFrame struct {
	Sequence       uint64          `json:"sequence"`
	CaseID         string          `json:"case_id"`
	Revision       int64           `json:"revision"`
	Type           string          `json:"type"`
	ActorID        string          `json:"actor_id"`
	Data           json.RawMessage `json:"data,omitempty"`
	OccurredAt     time.Time       `json:"occurred_at"`
	PreviousDigest string          `json:"previous_digest"`
}

type StoredResult struct {
	RequestID   string          `json:"request_id"`
	Fingerprint string          `json:"fingerprint"`
	HTTPStatus  int             `json:"http_status"`
	Body        json.RawMessage `json:"body"`
	CreatedAt   time.Time       `json:"created_at"`
}

type CommitInput struct {
	Case             *domain.DrillCase
	ExpectedRevision int64
	Event            Event
	Result           *StoredResult
}

type Integrity struct {
	Healthy      bool   `json:"healthy"`
	Frames       uint64 `json:"frames"`
	LastDigest   string `json:"last_digest"`
	ErrorMessage string `json:"error,omitempty"`
}
