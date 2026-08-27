package domain

import (
	"math"
	"testing"
	"time"
)

func preparedCase(t *testing.T) *DrillCase {
	t.Helper()
	now := time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC)
	c, err := NewCase("case-test", "丙酮泄漏演练", "实验楼 A", "coord", []string{"observer"}, now)
	if err != nil {
		t.Fatal(err)
	}
	err = c.FreezeBaseline(BaselineInput{ChemicalName: "丙酮", HazardClass: "易燃", AffectedZones: []string{"A201"}, RequiredRoles: []string{"警戒"}, ObservationPoints: []string{"alarm", "evac"}, Thresholds: []Threshold{{PointID: "alarm", Label: "报警", Rule: "lte", Target: 60, Unit: "秒"}, {PointID: "evac", Label: "疏散", Rule: "gte", Target: 100, Unit: "%"}}}, "coord", now)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range RequiredCheckItems {
		if err := c.RecordPreflight(item, true, "证据", "coord", now); err != nil {
			t.Fatal(err)
		}
	}
	if c.Status != StatusReady {
		t.Fatalf("status = %s", c.Status)
	}
	return c
}

func TestBaselinePrecheckNormalizesAndReportsConflicts(t *testing.T) {
	check := NormalizeBaseline(BaselineInput{ChemicalName: " 丙酮 ", HazardClass: "易燃", AffectedZones: []string{"A"}, RequiredRoles: []string{"警戒"}, ObservationPoints: []string{" p1 ", "p1"}, Thresholds: []Threshold{{PointID: "p1", Label: "时间", Unit: "秒", Rule: "lte", Target: math.Inf(1)}}})
	if len(check.Issues) < 2 || check.CandidateDigest != "" {
		t.Fatalf("unexpected precheck: %#v", check)
	}
	valid := NormalizeBaseline(BaselineInput{ChemicalName: " 丙酮 ", HazardClass: "易燃", AffectedZones: []string{"A"}, RequiredRoles: []string{"警戒"}, ObservationPoints: []string{" p2 ", "p1"}, Thresholds: []Threshold{{PointID: "p1", Label: "时间", Unit: "秒", Rule: "lte", Target: 60}, {PointID: "p2", Label: "比例", Unit: "%", Rule: "gte", Target: 90}}})
	if len(valid.Issues) != 0 || valid.CandidateDigest == "" || valid.Normalized.ObservationPoints[0] != "p1" {
		t.Fatalf("valid precheck failed: %#v", valid)
	}
	c, _ := NewCase("case-precheck", "演练", "A", "coord", []string{"observer"}, time.Now().UTC())
	if err := c.FreezeBaseline(valid.Normalized, "coord", time.Now().UTC()); err != nil || c.Baseline.ContentDigest != valid.CandidateDigest {
		t.Fatalf("preview/freeze digest mismatch: %v %s %s", err, c.Baseline.ContentDigest, valid.CandidateDigest)
	}
}

func TestExpiredPreflightBlocksStartAndCorrectionsUseLastValue(t *testing.T) {
	now := time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC)
	c, _ := NewCase("case-extra", "演练", "A", "coord", []string{"observer"}, now)
	_ = c.FreezeBaseline(BaselineInput{ChemicalName: "x", HazardClass: "y", AffectedZones: []string{"A"}, RequiredRoles: []string{"r"}, ObservationPoints: []string{"p"}, Thresholds: []Threshold{{PointID: "p", Label: "p", Unit: "u", Rule: "lte", Target: 10}}}, "coord", now)
	for _, item := range RequiredCheckItems {
		_ = c.RecordPreflight(item, true, "证据", "coord", now.Add(time.Hour), now)
	}
	if c.StartSession("initial", "observer", now.Add(2*time.Hour)) == nil {
		t.Fatal("expired checks should block start")
	}
	for _, item := range RequiredCheckItems {
		_ = c.RecordPreflight(item, true, "证据", "coord", now.Add(24*time.Hour), now)
	}
	if err := c.StartSession("initial", "observer", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := c.RecordObservation("p", 20, "原值", "observer", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	value := 5.0
	session := c.Sessions[0]
	if err := c.CorrectSessionRecord(session.SessionID, "observation_value", 0, "p", "", &value, "录入错误", "observer", now.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := c.FinishSession("observer", now.Add(3*time.Hour)); err != nil || len(c.Deviations) != 0 {
		t.Fatalf("corrected value not used: %v %#v", err, c.Deviations)
	}
}

func TestWorkflowFailureRetestAndApproval(t *testing.T) {
	c := preparedCase(t)
	now := time.Date(2026, 8, 27, 9, 0, 0, 0, time.UTC)
	if err := c.StartSession("initial", "observer", now); err != nil {
		t.Fatal(err)
	}
	if err := c.RecordEvent("discovery", "发现泄漏", "observer", 1, now); err != nil {
		t.Fatal(err)
	}
	if err := c.RecordObservation("alarm", 75, "计时", "observer", now); err != nil {
		t.Fatal(err)
	}
	if err := c.RecordObservation("evac", 100, "清点", "observer", now); err != nil {
		t.Fatal(err)
	}
	if err := c.FinishSession("observer", now); err != nil {
		t.Fatal(err)
	}
	if c.Status != StatusRemediation || len(c.Deviations) != 1 {
		t.Fatalf("unexpected failure result: %s %#v", c.Status, c.Deviations)
	}
	deviation := c.Deviations[0]
	if deviation.ObservationPointID != "alarm" {
		t.Fatalf("unexpected failed point %s", deviation.ObservationPointID)
	}
	if err := c.Remediate(deviation.DeviationID, "不熟练", "专项训练", "owner", now.Add(time.Hour), "证据摘要", "coord"); err != nil {
		t.Fatal(err)
	}
	if err := c.StartSession("retest", "observer", now); err != nil {
		t.Fatal(err)
	}
	session, _ := c.ActiveSession()
	if len(session.ScopePointIDs) != 1 || session.ScopePointIDs[0] != "alarm" {
		t.Fatalf("scope expanded: %#v", session.ScopePointIDs)
	}
	if err := c.RecordObservation("evac", 100, "范围外", "observer", now); ErrorCode(err) != CodeValidation {
		t.Fatalf("expected out-of-scope validation, got %v", err)
	}
	if err := c.RecordObservation("alarm", 50, "复测", "observer", now); err != nil {
		t.Fatal(err)
	}
	if err := c.FinishSession("observer", now); err != nil {
		t.Fatal(err)
	}
	if err := c.Review("approve", "coord", "意见", now); ErrorCode(err) != CodeForbidden {
		t.Fatalf("expected role separation, got %v", err)
	}
	if err := c.Review("approve", "reviewer", "证据完整", now); err != nil {
		t.Fatal(err)
	}
	if c.Status != StatusApproved {
		t.Fatalf("status = %s", c.Status)
	}
	valid, computed, err := VerifyDossier(c.Dossier)
	if err != nil || !valid || computed != c.Dossier.ContentDigest {
		t.Fatalf("dossier invalid: %v %t", err, valid)
	}
	if err := c.RecordPreflight(RequiredCheckItems[0], true, "改写", "coord", now); ErrorCode(err) != CodeState {
		t.Fatalf("terminal case mutated: %v", err)
	}
}

func TestBaselineValidationAndFreeze(t *testing.T) {
	now := time.Now().UTC()
	c, err := NewCase("case-freeze", "演练", "A", "coord", []string{"observer"}, now)
	if err != nil {
		t.Fatal(err)
	}
	bad := BaselineInput{ChemicalName: "甲醇", HazardClass: "易燃", AffectedZones: []string{"A"}, RequiredRoles: []string{"警戒"}, ObservationPoints: []string{"p1"}, Thresholds: []Threshold{{PointID: "p2", Label: "响应时间", Unit: "秒", Rule: "lte", Target: 1}}}
	if err := c.FreezeBaseline(bad, "coord", now); ErrorCode(err) != CodeValidation {
		t.Fatalf("expected validation, got %v", err)
	}
	bad.Thresholds[0].PointID = "p1"
	if err := c.FreezeBaseline(bad, "coord", now); err != nil {
		t.Fatal(err)
	}
	digest := c.Baseline.ContentDigest
	if err := c.FreezeBaseline(bad, "coord", now); ErrorCode(err) != CodeState {
		t.Fatalf("expected frozen state, got %v", err)
	}
	if c.Baseline.ContentDigest != digest {
		t.Fatal("digest changed")
	}
}
