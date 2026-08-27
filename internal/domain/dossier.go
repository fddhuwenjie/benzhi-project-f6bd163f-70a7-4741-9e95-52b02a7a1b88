package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

type ManifestCheck struct {
	Item       string    `json:"item"`
	Passed     bool      `json:"passed"`
	Evidence   string    `json:"evidence_summary"`
	CheckedBy  string    `json:"checked_by"`
	CheckedAt  time.Time `json:"checked_at"`
	ValidUntil time.Time `json:"valid_until"`
}

type ManifestSession struct {
	SessionID    string              `json:"session_id"`
	Kind         string              `json:"kind"`
	Scope        []string            `json:"scope"`
	StartedAt    time.Time           `json:"started_at"`
	EndedAt      *time.Time          `json:"ended_at,omitempty"`
	Outcome      string              `json:"outcome"`
	Events       []TimelineEvent     `json:"events"`
	Observations []Observation       `json:"observations"`
	Corrections  []SessionCorrection `json:"corrections"`
}

type ManifestDeviation struct {
	PointID          string    `json:"point_id"`
	MeasuredValue    float64   `json:"measured_value"`
	Threshold        Threshold `json:"threshold"`
	Cause            string    `json:"cause"`
	CorrectiveAction string    `json:"corrective_action"`
	OwnerID          string    `json:"owner_id"`
	DueAt            time.Time `json:"due_at"`
	EvidenceDigest   string    `json:"evidence_digest"`
	Status           string    `json:"status"`
}

type DossierManifest struct {
	Schema          string                `json:"schema"`
	CaseID          string                `json:"case_id"`
	Title           string                `json:"title"`
	Building        string                `json:"building"`
	CoordinatorID   string                `json:"coordinator_id"`
	ObserverIDs     []string              `json:"observer_ids"`
	Baseline        ScenarioBaseline      `json:"baseline"`
	Checks          []ManifestCheck       `json:"checks"`
	Sessions        []ManifestSession     `json:"sessions"`
	Deviations      []ManifestDeviation   `json:"deviations"`
	Decision        string                `json:"decision"`
	ReviewerID      string                `json:"reviewer_id"`
	ReviewNote      string                `json:"review_note"`
	ReviewChecklist []ReviewChecklistItem `json:"review_checklist"`
}

var RequiredReviewChecklist = []string{"scenario_summary", "preflight_evidence", "complete_timeline", "deviation_evidence", "retest_conclusion", "role_independence"}

func (c *DrillCase) Review(decision, reviewer, note string, args ...any) error {
	checklist := []ReviewChecklistItem{}
	now := time.Now().UTC()
	for _, arg := range args {
		switch value := arg.(type) {
		case []ReviewChecklistItem:
			checklist = value
		case time.Time:
			now = value
		}
	}
	if len(checklist) == 0 {
		for _, item := range RequiredReviewChecklist {
			checklist = append(checklist, ReviewChecklistItem{Item: item, Passed: true})
		}
	}
	if c.Status != StatusReview {
		return NewError(CodeState, "仅待复核案件可以作出复核结论")
	}
	reviewer, note = strings.TrimSpace(reviewer), strings.TrimSpace(note)
	if reviewer == "" || note == "" {
		return NewError(CodeValidation, "复核员和复核意见不能为空")
	}
	if reviewer == c.CoordinatorID || contains(c.ObserverIDs, reviewer) {
		return NewError(CodeForbidden, "复核员必须独立于协调员和观察员")
	}
	if decision != "approve" && decision != "reject" {
		return NewError(CodeValidation, "复核结论必须为 approve 或 reject")
	}
	items, err := validateReviewChecklist(checklist)
	if err != nil {
		return err
	}
	if decision == "approve" {
		for _, item := range items {
			if !item.Passed {
				return NewError(CodeState, "复核清单未通过：%s", item.Item)
			}
		}
	} else {
		failed := false
		for _, item := range items {
			failed = failed || !item.Passed
		}
		if !failed {
			return NewError(CodeValidation, "拒绝时至少需要一个不通过的复核清单项")
		}
	}
	c.ReviewerID = reviewer
	manifest := c.BuildManifest(decision, reviewer, note)
	manifest.ReviewChecklist = items
	digest, err := DigestManifest(manifest)
	if err != nil {
		return err
	}
	c.Dossier = &ReadinessDossier{DossierID: NewID("dossier"), CaseID: c.CaseID, Decision: decision, ReviewerID: reviewer, ReviewNote: note, Manifest: manifest, ContentDigest: digest, SealedAt: now.UTC()}
	closed := now.UTC()
	c.ClosedAt = &closed
	if decision == "approve" {
		c.Status = StatusApproved
	} else {
		c.Status = StatusRejected
	}
	return nil
}

func validateReviewChecklist(checklist []ReviewChecklistItem) ([]ReviewChecklistItem, error) {
	byItem := map[string]ReviewChecklistItem{}
	for _, item := range checklist {
		if !contains(RequiredReviewChecklist, item.Item) || byItem[item.Item].Item != "" {
			return nil, NewError(CodeValidation, "复核清单存在无效或重复项")
		}
		item.Note = strings.TrimSpace(item.Note)
		byItem[item.Item] = item
	}
	if len(byItem) != len(RequiredReviewChecklist) {
		missing := []string{}
		for _, item := range RequiredReviewChecklist {
			if _, ok := byItem[item]; !ok {
				missing = append(missing, item)
			}
		}
		return nil, NewError(CodeValidation, "复核清单缺少：%s", strings.Join(missing, "、"))
	}
	items := make([]ReviewChecklistItem, 0, len(RequiredReviewChecklist))
	for _, name := range RequiredReviewChecklist {
		items = append(items, byItem[name])
	}
	return items, nil
}

func (c *DrillCase) BuildManifest(decision, reviewer, note string) DossierManifest {
	manifest := DossierManifest{Schema: "readiness-dossier/v1", CaseID: c.CaseID, Title: c.Title, Building: c.Building, CoordinatorID: c.CoordinatorID, ObserverIDs: append([]string(nil), c.ObserverIDs...), Decision: decision, ReviewerID: reviewer, ReviewNote: note}
	if c.Baseline != nil {
		manifest.Baseline = *c.Baseline
	}
	manifest.Baseline.FrozenAt = manifest.Baseline.FrozenAt.UTC()
	for _, check := range c.Preflight {
		manifest.Checks = append(manifest.Checks, ManifestCheck{Item: check.Item, Passed: check.Passed, Evidence: check.Evidence, CheckedBy: check.CheckedBy, CheckedAt: check.CheckedAt.UTC(), ValidUntil: check.ValidUntil.UTC()})
	}
	for _, session := range c.Sessions {
		manifest.Sessions = append(manifest.Sessions, ManifestSession{SessionID: session.SessionID, Kind: session.SessionKind, Scope: append([]string(nil), session.ScopePointIDs...), StartedAt: session.StartedAt.UTC(), EndedAt: session.EndedAt, Outcome: session.Outcome, Events: append([]TimelineEvent(nil), session.EventSequence...), Observations: append([]Observation(nil), session.Observations...), Corrections: append([]SessionCorrection(nil), session.Corrections...)})
	}
	for _, deviation := range c.Deviations {
		manifest.Deviations = append(manifest.Deviations, ManifestDeviation{PointID: deviation.ObservationPointID, MeasuredValue: deviation.MeasuredValue, Threshold: deviation.ThresholdSnapshot, Cause: deviation.Cause, CorrectiveAction: deviation.CorrectiveAction, OwnerID: deviation.OwnerID, DueAt: deviation.DueAt.UTC(), EvidenceDigest: deviation.EvidenceDigest, Status: deviation.Status})
	}
	sort.Strings(manifest.ObserverIDs)
	sort.Slice(manifest.Checks, func(i, j int) bool { return manifest.Checks[i].Item < manifest.Checks[j].Item })
	sort.Slice(manifest.Deviations, func(i, j int) bool { return manifest.Deviations[i].PointID < manifest.Deviations[j].PointID })
	return manifest
}

func DigestManifest(manifest DossierManifest) (string, error) {
	payload, err := json.Marshal(manifest)
	if err != nil {
		return "", NewError(CodeIntegrity, "无法规范化就绪档案: %v", err)
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func VerifyDossier(dossier *ReadinessDossier) (bool, string, error) {
	if dossier == nil {
		return false, "", NewError(CodeNotFound, "案件尚未生成就绪档案")
	}
	digest, err := DigestManifest(dossier.Manifest)
	if err != nil {
		return false, "", err
	}
	return digest == dossier.ContentDigest, digest, nil
}
