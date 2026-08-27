package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

var RequiredCheckItems = []string{"personal_protection", "isolation_equipment", "evacuation_route", "communications", "observer_assignment"}

func (c *DrillCase) PreflightBlockers(now time.Time) []string {
	byItem := map[string]PreflightCheck{}
	for _, check := range c.Preflight {
		byItem[check.Item] = check
	}
	blockers := []string{}
	for _, item := range RequiredCheckItems {
		check, ok := byItem[item]
		if !ok || !check.Passed {
			blockers = append(blockers, item+" 未合格")
		} else if !now.Before(check.ValidUntil) {
			blockers = append(blockers, item+" 已失效")
		}
	}
	return blockers
}

func (c *DrillCase) DecorateTemporal(now time.Time) {
	c.StartBlockers = c.PreflightBlockers(now)
	for i := range c.Preflight {
		if c.Preflight[i].ValidUntil.IsZero() || !now.Before(c.Preflight[i].ValidUntil) {
			c.Preflight[i].ValidityStatus = "expired"
		} else if c.Preflight[i].ValidUntil.Sub(now) <= 24*time.Hour {
			c.Preflight[i].ValidityStatus = "expiring"
		} else {
			c.Preflight[i].ValidityStatus = "valid"
		}
	}
	summary := &DeviationSummary{}
	for i := range c.Deviations {
		d := &c.Deviations[i]
		switch d.Status {
		case "verified":
			summary.Verified++
			d.GovernanceStatus = "verified"
		case "open":
			summary.PendingMaterials++
			d.GovernanceStatus = "pending_materials"
		case "remediated":
			if d.DueAt.IsZero() || !now.Before(d.DueAt) {
				summary.Overdue++
				d.GovernanceStatus = "overdue"
			} else {
				summary.Registered++
				d.GovernanceStatus = "registered"
			}
		}
	}
	c.DeviationSummary = summary
}

type BaselineInput struct {
	ChemicalName      string      `json:"chemical_name"`
	HazardClass       string      `json:"hazard_class"`
	AffectedZones     []string    `json:"affected_zones"`
	RequiredRoles     []string    `json:"required_roles"`
	ObservationPoints []string    `json:"observation_points"`
	Thresholds        []Threshold `json:"thresholds"`
}

type BaselineIssue struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type BaselinePrecheck struct {
	Issues          []BaselineIssue `json:"issues"`
	Normalized      BaselineInput   `json:"normalized"`
	CandidateDigest string          `json:"candidate_digest,omitempty"`
}

// NormalizeBaseline is the single validation and digest source used by both precheck and freeze.
func NormalizeBaseline(input BaselineInput) BaselinePrecheck {
	result := BaselinePrecheck{}
	result.Normalized = input
	result.Normalized.ChemicalName = strings.TrimSpace(input.ChemicalName)
	result.Normalized.HazardClass = strings.TrimSpace(input.HazardClass)
	result.Normalized.AffectedZones = uniqueStrings(input.AffectedZones)
	result.Normalized.RequiredRoles = uniqueStrings(input.RequiredRoles)
	result.Normalized.ObservationPoints = normalizedIDs(input.ObservationPoints)
	result.Normalized.Thresholds = append([]Threshold(nil), input.Thresholds...)
	if result.Normalized.ChemicalName == "" {
		result.Issues = append(result.Issues, BaselineIssue{"chemical_name", "required", "泄漏物质不能为空"})
	}
	if result.Normalized.HazardClass == "" {
		result.Issues = append(result.Issues, BaselineIssue{"hazard_class", "required", "危险类别不能为空"})
	}
	if len(result.Normalized.AffectedZones) == 0 {
		result.Issues = append(result.Issues, BaselineIssue{"affected_zones", "required", "影响区域不能为空"})
	}
	if len(result.Normalized.RequiredRoles) == 0 {
		result.Issues = append(result.Issues, BaselineIssue{"required_roles", "required", "响应角色不能为空"})
	}
	if len(result.Normalized.ObservationPoints) == 0 {
		result.Issues = append(result.Issues, BaselineIssue{"observation_points", "required", "观测点不能为空"})
	}
	for i, point := range result.Normalized.ObservationPoints {
		if point == "" {
			result.Issues = append(result.Issues, BaselineIssue{fmtField("observation_points", i), "required", "观测点标识不能为空"})
		}
	}
	checkUniqueNormalized(&result, "observation_points", result.Normalized.ObservationPoints)
	if len(result.Normalized.Thresholds) != len(result.Normalized.ObservationPoints) {
		result.Issues = append(result.Issues, BaselineIssue{"thresholds", "coverage", "每个观测点必须且只能配置一个阈值"})
	}
	seenIDs := map[string]bool{}
	seenLabels := map[string]bool{}
	for i := range result.Normalized.Thresholds {
		t := &result.Normalized.Thresholds[i]
		t.PointID = strings.TrimSpace(t.PointID)
		t.Label = strings.TrimSpace(t.Label)
		t.Unit = strings.TrimSpace(t.Unit)
		if !contains(result.Normalized.ObservationPoints, t.PointID) {
			result.Issues = append(result.Issues, BaselineIssue{fmtField("thresholds", i), "point", "阈值观测点未覆盖或不在观测点范围内"})
		}
		if seenIDs[t.PointID] {
			result.Issues = append(result.Issues, BaselineIssue{fmtField("thresholds", i), "duplicate", "阈值标识规范化后重复"})
		}
		if t.Label != "" && seenLabels[t.Label] {
			result.Issues = append(result.Issues, BaselineIssue{fmtField("thresholds", i), "duplicate", "阈值显示名称规范化后重复"})
		}
		seenIDs[t.PointID], seenLabels[t.Label] = true, t.Label != ""
		if t.Label == "" {
			result.Issues = append(result.Issues, BaselineIssue{fmtField("thresholds", i), "required", "阈值显示名称不能为空"})
		}
		if t.Unit == "" {
			result.Issues = append(result.Issues, BaselineIssue{fmtField("thresholds", i), "required", "阈值单位不能为空"})
		}
		if !math.IsNaN(t.Target) && !math.IsInf(t.Target, 0) {
		} else {
			result.Issues = append(result.Issues, BaselineIssue{fmtField("thresholds", i), "finite", "阈值目标必须是有限数值"})
		}
		if t.Rule != "lte" && t.Rule != "gte" {
			result.Issues = append(result.Issues, BaselineIssue{fmtField("thresholds", i), "rule", "阈值规则必须为 lte 或 gte"})
		}
	}
	for i, point := range result.Normalized.ObservationPoints {
		if !seenIDs[point] {
			result.Issues = append(result.Issues, BaselineIssue{fmtField("observation_points", i), "uncovered", "观测点未配置对应阈值"})
		}
	}
	if len(result.Issues) == 0 {
		result.Normalized.ObservationPoints = uniqueStrings(result.Normalized.ObservationPoints)
		sort.Slice(result.Normalized.Thresholds, func(i, j int) bool {
			return result.Normalized.Thresholds[i].PointID < result.Normalized.Thresholds[j].PointID
		})
		result.CandidateDigest = baselineDigest(result.Normalized)
	}
	return result
}

func normalizedIDs(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		out = append(out, strings.TrimSpace(v))
	}
	return out
}
func checkUniqueNormalized(result *BaselinePrecheck, field string, values []string) {
	seen := map[string]bool{}
	for i, v := range values {
		if v == "" {
			continue
		}
		if seen[v] {
			result.Issues = append(result.Issues, BaselineIssue{fmtField(field, i), "duplicate", "标识规范化后重复"})
		}
		seen[v] = true
	}
}
func fmtField(field string, index int) string { return field + "[" + strconv.Itoa(index) + "]" }
func baselineDigest(input BaselineInput) string {
	payload, _ := json.Marshal(input)
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func (c *DrillCase) EnsureMutable() error {
	if c.Status.Terminal() {
		return NewError(CodeState, "案件已冻结为终态，不允许修改")
	}
	return nil
}

func (c *DrillCase) FreezeBaseline(input BaselineInput, actor string, now time.Time) error {
	if err := c.EnsureMutable(); err != nil {
		return err
	}
	if c.Status != StatusDraft {
		return NewError(CodeState, "仅草拟态可以冻结情景")
	}
	if actor != c.CoordinatorID {
		return NewError(CodeForbidden, "仅案件协调员可以冻结情景")
	}
	check := NormalizeBaseline(input)
	if len(check.Issues) > 0 {
		return NewError(CodeValidation, "%s", check.Issues[0].Message)
	}
	input = check.Normalized
	thresholds := input.Thresholds
	baseline := &ScenarioBaseline{BaselineID: NewID("baseline"), CaseID: c.CaseID, ChemicalName: input.ChemicalName, HazardClass: input.HazardClass, AffectedZones: input.AffectedZones, RequiredRoles: input.RequiredRoles, ObservationPoints: input.ObservationPoints, Thresholds: thresholds, FrozenAt: now.UTC()}
	baseline.ContentDigest = check.CandidateDigest
	c.Baseline = baseline
	c.Status = StatusPendingCheck
	return nil
}

func (c *DrillCase) RecordPreflight(item string, passed bool, evidence, actor string, times ...time.Time) error {
	now := time.Now().UTC()
	validUntil := time.Time{}
	if len(times) == 1 {
		now = times[0]
	} else if len(times) >= 2 {
		validUntil, now = times[0], times[1]
	}
	if validUntil.IsZero() {
		validUntil = now.Add(24 * time.Hour)
	}
	if err := c.EnsureMutable(); err != nil {
		return err
	}
	if c.Status != StatusPendingCheck && c.Status != StatusReady {
		return NewError(CodeState, "当前状态不能记录开始前核验")
	}
	if actor != c.CoordinatorID {
		return NewError(CodeForbidden, "仅案件协调员可以确认核验")
	}
	if !contains(RequiredCheckItems, item) || strings.TrimSpace(evidence) == "" {
		return NewError(CodeValidation, "核验项目无效或证据摘要为空")
	}
	if validUntil.IsZero() || !validUntil.After(now) {
		return NewError(CodeValidation, "核验有效截止时间必须晚于核验时间")
	}
	check := PreflightCheck{Item: item, Passed: passed, Evidence: strings.TrimSpace(evidence), CheckedBy: actor, CheckedAt: now.UTC(), LastConfirmedAt: now.UTC(), ValidUntil: validUntil.UTC()}
	replaced := false
	for i := range c.Preflight {
		if c.Preflight[i].Item == item {
			c.Preflight[i], replaced = check, true
		}
	}
	if !replaced {
		c.Preflight = append(c.Preflight, check)
	}
	sort.Slice(c.Preflight, func(i, j int) bool { return c.Preflight[i].Item < c.Preflight[j].Item })
	c.Status = StatusPendingCheck
	if len(c.Preflight) == len(RequiredCheckItems) {
		allPassed := true
		for _, current := range c.Preflight {
			allPassed = allPassed && current.Passed
		}
		if allPassed {
			c.Status = StatusReady
		}
	}
	return nil
}
