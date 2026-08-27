package domain

import (
	"crypto/rand"
	"encoding/hex"
	"sort"
	"strings"
	"time"
)

type Status string

const (
	StatusDraft        Status = "draft"
	StatusPendingCheck Status = "pending_check"
	StatusReady        Status = "ready"
	StatusRunning      Status = "running"
	StatusRemediation  Status = "remediation"
	StatusRetestReady  Status = "retest_ready"
	StatusRetestRun    Status = "retest_running"
	StatusReview       Status = "pending_review"
	StatusApproved     Status = "approved"
	StatusRejected     Status = "rejected"
)

func (s Status) Terminal() bool { return s == StatusApproved || s == StatusRejected }

type Threshold struct {
	PointID string  `json:"point_id"`
	Label   string  `json:"label"`
	Unit    string  `json:"unit"`
	Rule    string  `json:"rule"`
	Target  float64 `json:"target"`
}

type ScenarioBaseline struct {
	BaselineID        string      `json:"baseline_id"`
	CaseID            string      `json:"case_id"`
	ChemicalName      string      `json:"chemical_name"`
	HazardClass       string      `json:"hazard_class"`
	AffectedZones     []string    `json:"affected_zones"`
	RequiredRoles     []string    `json:"required_roles"`
	ObservationPoints []string    `json:"observation_points"`
	Thresholds        []Threshold `json:"thresholds"`
	FrozenAt          time.Time   `json:"frozen_at"`
	ContentDigest     string      `json:"content_digest"`
}

type PreflightCheck struct {
	Item            string    `json:"item"`
	Passed          bool      `json:"passed"`
	Evidence        string    `json:"evidence_summary"`
	CheckedBy       string    `json:"checked_by"`
	CheckedAt       time.Time `json:"checked_at"`
	ValidUntil      time.Time `json:"valid_until"`
	ValidityStatus  string    `json:"validity_status,omitempty"`
	LastConfirmedAt time.Time `json:"last_confirmed_at"`
}

type TimelineEvent struct {
	Sequence   int       `json:"sequence"`
	Action     string    `json:"action"`
	ActorID    string    `json:"actor_id"`
	Note       string    `json:"note"`
	OccurredAt time.Time `json:"occurred_at"`
}

type Observation struct {
	PointID    string    `json:"point_id"`
	Value      float64   `json:"value"`
	Evidence   string    `json:"evidence_summary"`
	ObservedAt time.Time `json:"observed_at"`
}

type SessionCorrection struct {
	Sequence      int       `json:"sequence"`
	TargetType    string    `json:"target_type"`
	EventSequence int       `json:"event_sequence,omitempty"`
	PointID       string    `json:"point_id,omitempty"`
	OriginalNote  string    `json:"original_note,omitempty"`
	NewNote       string    `json:"new_note,omitempty"`
	OriginalValue *float64  `json:"original_value,omitempty"`
	NewValue      *float64  `json:"new_value,omitempty"`
	Reason        string    `json:"reason"`
	CorrectedBy   string    `json:"corrected_by"`
	CorrectedAt   time.Time `json:"corrected_at"`
}

type DrillSession struct {
	SessionID     string              `json:"session_id"`
	CaseID        string              `json:"case_id"`
	SessionKind   string              `json:"session_kind"`
	ScopePointIDs []string            `json:"scope_point_ids"`
	StartedAt     time.Time           `json:"started_at"`
	EndedAt       *time.Time          `json:"ended_at,omitempty"`
	EventSequence []TimelineEvent     `json:"event_sequence"`
	Observations  []Observation       `json:"observations"`
	Corrections   []SessionCorrection `json:"corrections"`
	Outcome       string              `json:"outcome"`
}

type Deviation struct {
	DeviationID        string    `json:"deviation_id"`
	CaseID             string    `json:"case_id"`
	ObservationPointID string    `json:"observation_point_id"`
	MeasuredValue      float64   `json:"measured_value"`
	ThresholdSnapshot  Threshold `json:"threshold_snapshot"`
	Cause              string    `json:"cause"`
	CorrectiveAction   string    `json:"corrective_action"`
	OwnerID            string    `json:"owner_id"`
	DueAt              time.Time `json:"due_at"`
	EvidenceDigest     string    `json:"evidence_digest"`
	Status             string    `json:"status"`
	GovernanceStatus   string    `json:"governance_status,omitempty"`
}

type ReviewChecklistItem struct {
	Item   string `json:"item"`
	Passed bool   `json:"passed"`
	Note   string `json:"note,omitempty"`
}

type ReadinessDossier struct {
	DossierID       string                `json:"dossier_id"`
	CaseID          string                `json:"case_id"`
	Decision        string                `json:"decision"`
	ReviewerID      string                `json:"reviewer_id"`
	ReviewNote      string                `json:"review_note"`
	ReviewChecklist []ReviewChecklistItem `json:"review_checklist"`
	Manifest        DossierManifest       `json:"manifest"`
	ContentDigest   string                `json:"content_digest"`
	SealedAt        time.Time             `json:"sealed_at"`
}

type DeviationSummary struct {
	PendingMaterials int `json:"pending_materials"`
	Registered       int `json:"registered"`
	Overdue          int `json:"overdue"`
	Verified         int `json:"verified"`
}

type DrillCase struct {
	CaseID           string            `json:"case_id"`
	Title            string            `json:"title"`
	Building         string            `json:"building"`
	Status           Status            `json:"status"`
	Revision         int64             `json:"revision"`
	CoordinatorID    string            `json:"coordinator_id"`
	ObserverIDs      []string          `json:"observer_ids"`
	ReviewerID       string            `json:"reviewer_id,omitempty"`
	CreatedAt        time.Time         `json:"created_at"`
	ClosedAt         *time.Time        `json:"closed_at,omitempty"`
	Baseline         *ScenarioBaseline `json:"baseline,omitempty"`
	Preflight        []PreflightCheck  `json:"preflight_checks"`
	Sessions         []DrillSession    `json:"sessions"`
	Deviations       []Deviation       `json:"deviations"`
	Dossier          *ReadinessDossier `json:"dossier,omitempty"`
	StartBlockers    []string          `json:"start_blockers,omitempty"`
	DeviationSummary *DeviationSummary `json:"deviation_summary,omitempty"`
}

func NewID(prefix string) string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return prefix + "-" + time.Now().UTC().Format("20060102150405.000000000")
	}
	return prefix + "-" + hex.EncodeToString(b)
}

func NewCase(id, title, building, coordinator string, observers []string, now time.Time) (*DrillCase, error) {
	title, building, coordinator = strings.TrimSpace(title), strings.TrimSpace(building), strings.TrimSpace(coordinator)
	if id == "" {
		id = NewID("case")
	}
	if err := Require(title != "" && building != "" && coordinator != "", CodeValidation, "案件标题、建筑和协调员均不能为空"); err != nil {
		return nil, err
	}
	clean := uniqueStrings(observers)
	if len(clean) == 0 {
		return nil, NewError(CodeValidation, "至少需要一名观察员")
	}
	for _, observer := range clean {
		if observer == coordinator {
			return nil, NewError(CodeValidation, "协调员不能同时作为观察员")
		}
	}
	return &DrillCase{CaseID: id, Title: title, Building: building, Status: StatusDraft, Revision: 1, CoordinatorID: coordinator, ObserverIDs: clean, CreatedAt: now.UTC()}, nil
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
