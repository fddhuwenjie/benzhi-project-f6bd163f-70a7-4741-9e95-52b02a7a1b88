package application

import (
	"context"
	"time"

	"benzhi-project-f6bd163f-70a7-4741-9e95-52b02a7a1b88/internal/domain"
)

type CommandMeta struct {
	RequestID        string          `json:"request_id"`
	ExpectedRevision int64           `json:"expected_revision"`
	ActorID          string          `json:"actor_id"`
	Context          context.Context `json:"-"`
}

type Outcome struct {
	Case       *domain.DrillCase `json:"case,omitempty"`
	Error      *domain.Error     `json:"error,omitempty"`
	Replayed   bool              `json:"replayed"`
	HTTPStatus int               `json:"-"`
}

type CreateCaseCommand struct {
	CommandMeta
	CaseID        string   `json:"case_id,omitempty"`
	Title         string   `json:"title"`
	Building      string   `json:"building"`
	CoordinatorID string   `json:"coordinator_id"`
	ObserverIDs   []string `json:"observer_ids"`
}

type FreezeBaselineCommand struct {
	CommandMeta
	CaseID   string               `json:"case_id"`
	Baseline domain.BaselineInput `json:"baseline"`
}

type PreflightCommand struct {
	CommandMeta
	CaseID          string    `json:"case_id"`
	Item            string    `json:"item"`
	Passed          bool      `json:"passed"`
	EvidenceSummary string    `json:"evidence_summary"`
	ValidUntil      time.Time `json:"valid_until"`
}

type BaselinePrecheckCommand struct {
	CommandMeta
	CaseID   string               `json:"case_id"`
	Baseline domain.BaselineInput `json:"baseline"`
}
type BaselinePrecheckResult struct {
	CaseID   string                  `json:"case_id"`
	Revision int64                   `json:"revision"`
	Precheck domain.BaselinePrecheck `json:"precheck"`
}

type StartSessionCommand struct {
	CommandMeta
	CaseID string `json:"case_id"`
	Kind   string `json:"kind"`
}

type RecordEventCommand struct {
	CommandMeta
	CaseID   string `json:"case_id"`
	Sequence int    `json:"sequence"`
	Action   string `json:"action"`
	Note     string `json:"note"`
}

type RecordObservationCommand struct {
	CommandMeta
	CaseID          string  `json:"case_id"`
	PointID         string  `json:"point_id"`
	Value           float64 `json:"value"`
	EvidenceSummary string  `json:"evidence_summary"`
}

type CorrectionCommand struct {
	CommandMeta
	CaseID        string   `json:"case_id"`
	SessionID     string   `json:"session_id"`
	TargetType    string   `json:"target_type"`
	EventSequence int      `json:"event_sequence,omitempty"`
	PointID       string   `json:"point_id,omitempty"`
	NewNote       string   `json:"new_note,omitempty"`
	NewValue      *float64 `json:"new_value,omitempty"`
	Reason        string   `json:"reason"`
}

type FinishSessionCommand struct {
	CommandMeta
	CaseID string `json:"case_id"`
}

type RemediateCommand struct {
	CommandMeta
	CaseID           string    `json:"case_id"`
	DeviationID      string    `json:"deviation_id"`
	Cause            string    `json:"cause"`
	CorrectiveAction string    `json:"corrective_action"`
	OwnerID          string    `json:"owner_id"`
	DueAt            time.Time `json:"due_at"`
	EvidenceDigest   string    `json:"evidence_digest"`
}

type BatchRemediateItem struct {
	DeviationID      string `json:"deviation_id"`
	Cause            string `json:"cause"`
	CorrectiveAction string `json:"corrective_action"`
	EvidenceDigest   string `json:"evidence_digest"`
}
type BatchRemediateCommand struct {
	CommandMeta
	CaseID  string               `json:"case_id"`
	OwnerID string               `json:"owner_id"`
	DueAt   time.Time            `json:"due_at"`
	Items   []BatchRemediateItem `json:"items"`
}

type ReviewCommand struct {
	CommandMeta
	CaseID     string                       `json:"case_id"`
	Decision   string                       `json:"decision"`
	ReviewNote string                       `json:"review_note"`
	Checklist  []domain.ReviewChecklistItem `json:"checklist"`
}

type DossierDownload struct {
	Schema        string                 `json:"schema"`
	CaseID        string                 `json:"case_id"`
	SealedAt      time.Time              `json:"sealed_at"`
	Manifest      domain.DossierManifest `json:"manifest"`
	ContentDigest string                 `json:"content_digest"`
}
