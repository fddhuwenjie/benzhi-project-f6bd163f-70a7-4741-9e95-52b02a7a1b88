package web

import (
	"fmt"
	"net/http"

	"benzhi-project-f6bd163f-70a7-4741-9e95-52b02a7a1b88/internal/application"
)

func (s *Server) HandlePrecheckBaseline(w http.ResponseWriter, r *http.Request) {
	var command application.BaselinePrecheckCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	command.CaseID = r.PathValue("caseID")
	result, err := s.service.PrecheckBaseline(command)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, apiEnvelope{Data: result})
}

func (s *Server) HandleListCases(w http.ResponseWriter, r *http.Request) {
	cases, err := s.service.ListCases()
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, apiEnvelope{Data: cases})
}

func (s *Server) HandleGetCase(w http.ResponseWriter, r *http.Request) {
	c, err := s.service.GetCase(r.PathValue("caseID"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, apiEnvelope{Data: c})
}

func (s *Server) HandleGetAuditTrail(w http.ResponseWriter, r *http.Request) {
	events, err := s.service.GetAuditTrail(r.PathValue("caseID"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, apiEnvelope{Data: events})
}

func (s *Server) HandleCreateCase(w http.ResponseWriter, r *http.Request) {
	var command application.CreateCaseCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	command.Context = r.Context()
	if command.ActorID == "" {
		command.ActorID = command.CoordinatorID
	}
	outcome, err := s.service.CreateCase(command)
	writeOutcome(w, outcome, err)
}

func (s *Server) HandleFreezeBaseline(w http.ResponseWriter, r *http.Request) {
	var command application.FreezeBaselineCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	command.Context = r.Context()
	command.CaseID = r.PathValue("caseID")
	outcome, err := s.service.FreezeBaseline(command)
	writeOutcome(w, outcome, err)
}

func (s *Server) HandlePreflight(w http.ResponseWriter, r *http.Request) {
	var command application.PreflightCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	command.Context = r.Context()
	command.CaseID = r.PathValue("caseID")
	outcome, err := s.service.RecordPreflight(command)
	writeOutcome(w, outcome, err)
}

func (s *Server) HandleVerifyDossier(w http.ResponseWriter, r *http.Request) {
	verification, err := s.service.VerifyDossier(r.PathValue("caseID"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, apiEnvelope{Data: verification})
}

func (s *Server) HandleDownloadDossier(w http.ResponseWriter, r *http.Request) {
	payload, filename, err := s.service.DownloadDossier(r.PathValue("caseID"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(payload)))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
}
