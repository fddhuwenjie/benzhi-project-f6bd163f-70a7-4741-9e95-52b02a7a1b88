package web

import (
	"net/http"

	"benzhi-project-f6bd163f-70a7-4741-9e95-52b02a7a1b88/internal/application"
)

func (s *Server) HandleStartSession(w http.ResponseWriter, r *http.Request) {
	var command application.StartSessionCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	command.CaseID = r.PathValue("caseID")
	outcome, err := s.service.StartSession(command)
	writeOutcome(w, outcome, err)
}

func (s *Server) HandleRecordEvent(w http.ResponseWriter, r *http.Request) {
	var command application.RecordEventCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	command.CaseID = r.PathValue("caseID")
	outcome, err := s.service.RecordEvent(command)
	writeOutcome(w, outcome, err)
}

func (s *Server) HandleRecordObservation(w http.ResponseWriter, r *http.Request) {
	var command application.RecordObservationCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	command.CaseID = r.PathValue("caseID")
	outcome, err := s.service.RecordObservation(command)
	writeOutcome(w, outcome, err)
}

func (s *Server) HandleCorrectSessionRecord(w http.ResponseWriter, r *http.Request) {
	var command application.CorrectionCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	command.CaseID = r.PathValue("caseID")
	outcome, err := s.service.CorrectSessionRecord(command)
	writeOutcome(w, outcome, err)
}

func (s *Server) HandleFinishSession(w http.ResponseWriter, r *http.Request) {
	var command application.FinishSessionCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	command.CaseID = r.PathValue("caseID")
	outcome, err := s.service.FinishSession(command)
	writeOutcome(w, outcome, err)
}

func (s *Server) HandleRemediate(w http.ResponseWriter, r *http.Request) {
	var command application.RemediateCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	command.CaseID = r.PathValue("caseID")
	outcome, err := s.service.Remediate(command)
	writeOutcome(w, outcome, err)
}

func (s *Server) HandleBatchRemediate(w http.ResponseWriter, r *http.Request) {
	var command application.BatchRemediateCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	command.CaseID = r.PathValue("caseID")
	outcome, err := s.service.BatchRemediate(command)
	writeOutcome(w, outcome, err)
}

func (s *Server) HandleReview(w http.ResponseWriter, r *http.Request) {
	var command application.ReviewCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	command.CaseID = r.PathValue("caseID")
	outcome, err := s.service.Review(command)
	writeOutcome(w, outcome, err)
}

func (s *Server) HandleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, apiEnvelope{Data: map[string]string{"status": "ok"}})
}

func (s *Server) HandleReady(w http.ResponseWriter, r *http.Request) {
	integrity := s.service.StorageIntegrity()
	if !integrity.Healthy {
		writeJSON(w, http.StatusServiceUnavailable, apiEnvelope{Error: nil, Data: integrity})
		return
	}
	writeJSON(w, http.StatusOK, apiEnvelope{Data: integrity})
}
