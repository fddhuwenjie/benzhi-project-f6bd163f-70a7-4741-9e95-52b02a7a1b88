package application

import "benzhi-project-f6bd163f-70a7-4741-9e95-52b02a7a1b88/internal/domain"

func (s *Service) StartSession(command StartSessionCommand) (*Outcome, error) {
	return s.execute(command.CaseID, "session.started", command.CommandMeta, command, 200, func(c *domain.DrillCase) error {
		return c.StartSession(command.Kind, command.ActorID, s.now())
	})
}

func (s *Service) RecordEvent(command RecordEventCommand) (*Outcome, error) {
	return s.execute(command.CaseID, "session.event_recorded", command.CommandMeta, command, 200, func(c *domain.DrillCase) error {
		return c.RecordEvent(command.Action, command.Note, command.ActorID, command.Sequence, s.now())
	})
}

func (s *Service) RecordObservation(command RecordObservationCommand) (*Outcome, error) {
	return s.execute(command.CaseID, "session.observation_recorded", command.CommandMeta, command, 200, func(c *domain.DrillCase) error {
		return c.RecordObservation(command.PointID, command.Value, command.EvidenceSummary, command.ActorID, s.now())
	})
}

func (s *Service) CorrectSessionRecord(command CorrectionCommand) (*Outcome, error) {
	return s.execute(command.CaseID, "session.record_corrected", command.CommandMeta, command, 200, func(c *domain.DrillCase) error {
		return c.CorrectSessionRecord(command.SessionID, command.TargetType, command.EventSequence, command.PointID, command.NewNote, command.NewValue, command.Reason, command.ActorID, s.now())
	})
}

func (s *Service) FinishSession(command FinishSessionCommand) (*Outcome, error) {
	return s.execute(command.CaseID, "session.finished", command.CommandMeta, command, 200, func(c *domain.DrillCase) error {
		return c.FinishSession(command.ActorID, s.now())
	})
}

func (s *Service) Remediate(command RemediateCommand) (*Outcome, error) {
	return s.execute(command.CaseID, "deviation.remediated", command.CommandMeta, command, 200, func(c *domain.DrillCase) error {
		return c.BatchRemediate([]domain.RemediationItem{{DeviationID: command.DeviationID, Cause: command.Cause, CorrectiveAction: command.CorrectiveAction, EvidenceDigest: command.EvidenceDigest}}, command.OwnerID, command.DueAt, command.ActorID, s.now())
	})
}

func (s *Service) BatchRemediate(command BatchRemediateCommand) (*Outcome, error) {
	items := make([]domain.RemediationItem, len(command.Items))
	for i, item := range command.Items {
		items[i] = domain.RemediationItem{DeviationID: item.DeviationID, Cause: item.Cause, CorrectiveAction: item.CorrectiveAction, EvidenceDigest: item.EvidenceDigest}
	}
	return s.execute(command.CaseID, "deviations.remediated_batch", command.CommandMeta, command, 200, func(c *domain.DrillCase) error {
		return c.BatchRemediate(items, command.OwnerID, command.DueAt, command.ActorID, s.now())
	})
}
