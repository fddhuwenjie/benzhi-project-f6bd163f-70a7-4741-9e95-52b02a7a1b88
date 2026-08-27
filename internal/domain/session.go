package domain

import (
	"math"
	"sort"
	"strings"
	"time"
)

func (c *DrillCase) StartSession(kind, actor string, now time.Time) error {
	if err := c.EnsureMutable(); err != nil {
		return err
	}
	if !contains(c.ObserverIDs, actor) {
		return NewError(CodeForbidden, "仅指定观察员可以启动和记录场次")
	}
	var scope []string
	switch kind {
	case "initial":
		if c.Status != StatusReady {
			return NewError(CodeState, "仅已准备案件可以启动首演")
		}
		if blockers := c.PreflightBlockers(now); len(blockers) > 0 {
			return NewError(CodeState, "启动前核验失效：%s", strings.Join(blockers, "、"))
		}
		scope = append([]string(nil), c.Baseline.ObservationPoints...)
		c.Status = StatusRunning
	case "retest":
		if c.Status != StatusRetestReady {
			return NewError(CodeState, "仅待复演案件可以启动定向复演")
		}
		for _, deviation := range c.Deviations {
			if deviation.Status == "remediated" {
				scope = append(scope, deviation.ObservationPointID)
			}
		}
		scope = uniqueStrings(scope)
		if len(scope) == 0 {
			return NewError(CodeState, "没有可复演的失败观测点")
		}
		c.Status = StatusRetestRun
	default:
		return NewError(CodeValidation, "场次类型无效")
	}
	c.Sessions = append(c.Sessions, DrillSession{SessionID: NewID("session"), CaseID: c.CaseID, SessionKind: kind, ScopePointIDs: scope, StartedAt: now.UTC(), EventSequence: []TimelineEvent{}, Observations: []Observation{}, Corrections: []SessionCorrection{}})
	return nil
}

func (c *DrillCase) ActiveSession() (*DrillSession, error) {
	if len(c.Sessions) == 0 {
		return nil, NewError(CodeState, "尚未创建演练场次")
	}
	session := &c.Sessions[len(c.Sessions)-1]
	if session.EndedAt != nil {
		return nil, NewError(CodeState, "最近场次已结束")
	}
	return session, nil
}

func (c *DrillCase) RecordEvent(action, note, actor string, sequence int, now time.Time) error {
	if c.Status != StatusRunning && c.Status != StatusRetestRun {
		return NewError(CodeState, "当前没有运行中的场次")
	}
	if !contains(c.ObserverIDs, actor) {
		return NewError(CodeForbidden, "仅指定观察员可以记录事件")
	}
	allowed := []string{"discovery", "alarm", "isolation", "evacuation", "control", "cleanup"}
	if !contains(allowed, action) {
		return NewError(CodeValidation, "动作类型无效")
	}
	session, err := c.ActiveSession()
	if err != nil {
		return err
	}
	expected := len(session.EventSequence) + 1
	if sequence != expected {
		return NewError(CodeConflict, "事件序号必须为 %d", expected)
	}
	if now.Before(session.StartedAt) {
		return NewError(CodeValidation, "事件时间不能早于场次启动时间")
	}
	if len(session.EventSequence) > 0 && now.Before(session.EventSequence[len(session.EventSequence)-1].OccurredAt) {
		return NewError(CodeValidation, "事件时间必须保持单调")
	}
	session.EventSequence = append(session.EventSequence, TimelineEvent{Sequence: sequence, Action: action, ActorID: actor, Note: strings.TrimSpace(note), OccurredAt: now.UTC()})
	return nil
}

func (c *DrillCase) RecordObservation(pointID string, value float64, evidence, actor string, now time.Time) error {
	if c.Status != StatusRunning && c.Status != StatusRetestRun {
		return NewError(CodeState, "当前没有运行中的场次")
	}
	if !contains(c.ObserverIDs, actor) {
		return NewError(CodeForbidden, "仅指定观察员可以记录观测")
	}
	session, err := c.ActiveSession()
	if err != nil {
		return err
	}
	if !contains(session.ScopePointIDs, pointID) {
		return NewError(CodeValidation, "观测点不在本场次冻结范围内")
	}
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return NewError(CodeValidation, "观测值必须是有限数值")
	}
	if strings.TrimSpace(evidence) == "" {
		return NewError(CodeValidation, "观测证据摘要不能为空")
	}
	if now.Before(session.StartedAt) {
		return NewError(CodeValidation, "观测时间不能早于场次启动时间")
	}
	for _, observation := range session.Observations {
		if observation.PointID == pointID {
			return NewError(CodeValidation, "同一场次不能重复记录观测点")
		}
	}
	session.Observations = append(session.Observations, Observation{PointID: pointID, Value: value, Evidence: strings.TrimSpace(evidence), ObservedAt: now.UTC()})
	sort.Slice(session.Observations, func(i, j int) bool { return session.Observations[i].PointID < session.Observations[j].PointID })
	return nil
}

func (c *DrillCase) FinishSession(actor string, now time.Time) error {
	if c.Status != StatusRunning && c.Status != StatusRetestRun {
		return NewError(CodeState, "当前没有可结束的场次")
	}
	if !contains(c.ObserverIDs, actor) {
		return NewError(CodeForbidden, "仅指定观察员可以结束场次")
	}
	session, err := c.ActiveSession()
	if err != nil {
		return err
	}
	if len(session.Observations) != len(session.ScopePointIDs) {
		return NewError(CodeValidation, "必须记录冻结范围内的全部观测点")
	}
	if now.Before(session.StartedAt) {
		return NewError(CodeValidation, "结束时间不能早于场次启动时间")
	}
	failed, err := Evaluate(c.Baseline.Thresholds, session.EffectiveObservations(), session.ScopePointIDs)
	if err != nil {
		return err
	}
	ended := now.UTC()
	session.EndedAt = &ended
	if len(failed) == 0 {
		session.Outcome = "passed"
		if session.SessionKind == "retest" {
			for i := range c.Deviations {
				c.Deviations[i].Status = "verified"
			}
		}
		c.Status = StatusReview
		return nil
	}
	session.Outcome = "failed"
	if session.SessionKind == "initial" {
		c.Deviations = failed
	} else {
		failedIDs := map[string]bool{}
		for _, deviation := range failed {
			failedIDs[deviation.ObservationPointID] = true
		}
		for i := range c.Deviations {
			if failedIDs[c.Deviations[i].ObservationPointID] {
				c.Deviations[i].Status = "open"
				c.Deviations[i].Cause = ""
				c.Deviations[i].CorrectiveAction = ""
				c.Deviations[i].OwnerID = ""
				c.Deviations[i].EvidenceDigest = ""
			} else {
				c.Deviations[i].Status = "verified"
			}
		}
	}
	c.Status = StatusRemediation
	return nil
}

func (s *DrillSession) EffectiveObservations() []Observation {
	values := map[string]Observation{}
	for _, item := range s.Observations {
		values[item.PointID] = item
	}
	for _, correction := range s.Corrections {
		if correction.TargetType == "observation_value" && correction.NewValue != nil {
			if current, ok := values[correction.PointID]; ok {
				current.Value = *correction.NewValue
				values[correction.PointID] = current
			}
		}
	}
	out := make([]Observation, 0, len(values))
	for _, item := range values {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PointID < out[j].PointID })
	return out
}

func (c *DrillCase) CorrectSessionRecord(sessionID, targetType string, eventSequence int, pointID, newNote string, newValue *float64, reason, actor string, now time.Time) error {
	if c.Status != StatusRunning && c.Status != StatusRetestRun {
		return NewError(CodeState, "当前没有运行中的场次")
	}
	if !contains(c.ObserverIDs, actor) {
		return NewError(CodeForbidden, "仅指定观察员可以更正场次记录")
	}
	if strings.TrimSpace(reason) == "" {
		return NewError(CodeValidation, "更正原因不能为空")
	}
	session, err := c.ActiveSession()
	if err != nil {
		return err
	}
	if session.SessionID != sessionID {
		return NewError(CodeConflict, "只能更正当前运行场次")
	}
	correction := SessionCorrection{Sequence: len(session.Corrections) + 1, TargetType: targetType, EventSequence: eventSequence, PointID: strings.TrimSpace(pointID), Reason: strings.TrimSpace(reason), CorrectedBy: actor, CorrectedAt: now.UTC()}
	switch targetType {
	case "event_note":
		if eventSequence < 1 || eventSequence > len(session.EventSequence) {
			return NewError(CodeNotFound, "未找到要更正的动作记录")
		}
		if strings.TrimSpace(newNote) == "" {
			return NewError(CodeValidation, "新动作备注不能为空")
		}
		current := session.EventSequence[eventSequence-1]
		correction.OriginalNote, correction.NewNote = current.Note, strings.TrimSpace(newNote)
	case "observation_value":
		if newValue == nil || math.IsNaN(*newValue) || math.IsInf(*newValue, 0) {
			return NewError(CodeValidation, "新观测值必须是有限数值")
		}
		if !contains(session.ScopePointIDs, correction.PointID) {
			return NewError(CodeValidation, "观测点不在本场次冻结范围内")
		}
		var current *Observation
		for i := range session.Observations {
			if session.Observations[i].PointID == correction.PointID {
				current = &session.Observations[i]
				break
			}
		}
		if current == nil {
			return NewError(CodeNotFound, "未找到要更正的观测记录")
		}
		effective := session.EffectiveObservations()
		for _, item := range effective {
			if item.PointID == correction.PointID {
				value := item.Value
				correction.OriginalValue = &value
				break
			}
		}
		correction.NewValue = newValue
	default:
		return NewError(CodeValidation, "更正目标类型无效")
	}
	session.Corrections = append(session.Corrections, correction)
	return nil
}

func Evaluate(thresholds []Threshold, observations []Observation, scope []string) ([]Deviation, error) {
	byThreshold := map[string]Threshold{}
	for _, threshold := range thresholds {
		byThreshold[threshold.PointID] = threshold
	}
	byObservation := map[string]Observation{}
	for _, observation := range observations {
		byObservation[observation.PointID] = observation
	}
	failed := []Deviation{}
	for _, pointID := range uniqueStrings(scope) {
		threshold, ok := byThreshold[pointID]
		if !ok {
			return nil, NewError(CodeIntegrity, "观测点 %s 缺少冻结阈值", pointID)
		}
		observation, ok := byObservation[pointID]
		if !ok {
			return nil, NewError(CodeValidation, "观测点 %s 缺少结果", pointID)
		}
		passed := threshold.Rule == "lte" && observation.Value <= threshold.Target || threshold.Rule == "gte" && observation.Value >= threshold.Target
		if !passed {
			failed = append(failed, Deviation{DeviationID: NewID("deviation"), ObservationPointID: pointID, MeasuredValue: observation.Value, ThresholdSnapshot: threshold, Status: "open"})
		}
	}
	sort.Slice(failed, func(i, j int) bool { return failed[i].ObservationPointID < failed[j].ObservationPointID })
	return failed, nil
}

func (c *DrillCase) Remediate(deviationID, cause, action, owner string, dueAt time.Time, evidence, actor string) error {
	return c.BatchRemediate([]RemediationItem{{DeviationID: deviationID, Cause: cause, CorrectiveAction: action, EvidenceDigest: evidence}}, owner, dueAt, actor, dueAt.Add(-time.Nanosecond))
}

type RemediationItem struct{ DeviationID, Cause, CorrectiveAction, EvidenceDigest string }

func (c *DrillCase) BatchRemediate(items []RemediationItem, owner string, dueAt time.Time, actor string, now time.Time) error {
	if c.Status != StatusRemediation {
		return NewError(CodeState, "仅待整改案件可以登记整改")
	}
	if actor != c.CoordinatorID {
		return NewError(CodeForbidden, "仅案件协调员可以登记整改")
	}
	if len(items) == 0 || strings.TrimSpace(owner) == "" || dueAt.IsZero() || !dueAt.After(now) {
		return NewError(CodeValidation, "原因、纠正措施、责任人、期限和证据摘要均不能为空")
	}
	selected := map[string]bool{}
	for _, item := range items {
		if selected[item.DeviationID] || strings.TrimSpace(item.DeviationID) == "" || strings.TrimSpace(item.Cause) == "" || strings.TrimSpace(item.CorrectiveAction) == "" || strings.TrimSpace(item.EvidenceDigest) == "" {
			return NewError(CodeValidation, "批量整改存在缺少资料或重复的偏差")
		}
		selected[item.DeviationID] = true
		found := false
		for _, deviation := range c.Deviations {
			if deviation.DeviationID == item.DeviationID {
				found = true
				if deviation.Status != "open" {
					return NewError(CodeState, "偏差 %s 当前不可整改", item.DeviationID)
				}
				break
			}
		}
		if !found {
			return NewError(CodeNotFound, "未找到偏差 %s", item.DeviationID)
		}
	}
	for _, item := range items {
		for i := range c.Deviations {
			if c.Deviations[i].DeviationID == item.DeviationID {
				c.Deviations[i].Cause = strings.TrimSpace(item.Cause)
				c.Deviations[i].CorrectiveAction = strings.TrimSpace(item.CorrectiveAction)
				c.Deviations[i].OwnerID = strings.TrimSpace(owner)
				c.Deviations[i].DueAt = dueAt.UTC()
				c.Deviations[i].EvidenceDigest = strings.TrimSpace(item.EvidenceDigest)
				c.Deviations[i].Status = "remediated"
			}
		}
	}
	allDone := true
	for _, deviation := range c.Deviations {
		allDone = allDone && (deviation.Status == "remediated" || deviation.Status == "verified")
	}
	if allDone {
		c.Status = StatusRetestReady
	}
	return nil
}
