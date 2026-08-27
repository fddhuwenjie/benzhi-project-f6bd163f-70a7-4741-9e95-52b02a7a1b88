package web

import (
	"net/http"

	"benzhi-project-f6bd163f-70a7-4741-9e95-52b02a7a1b88/internal/application"
)

type Server struct {
	service *application.Service
	mux     *http.ServeMux
}

func NewServer(service *application.Service) *Server {
	s := &Server{service: service, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /", s.HandleWorkspace)
	s.mux.HandleFunc("GET /assets/app.css", s.HandleCSS)
	s.mux.HandleFunc("GET /assets/app.js", s.HandleJavaScript)
	s.mux.HandleFunc("GET /healthz", s.HandleHealth)
	s.mux.HandleFunc("GET /readyz", s.HandleReady)
	s.mux.HandleFunc("GET /api/cases", s.HandleListCases)
	s.mux.HandleFunc("POST /api/cases", s.HandleCreateCase)
	s.mux.HandleFunc("GET /api/cases/{caseID}", s.HandleGetCase)
	s.mux.HandleFunc("GET /api/cases/{caseID}/audit", s.HandleGetAuditTrail)
	s.mux.HandleFunc("POST /api/cases/{caseID}/baseline/freeze", s.HandleFreezeBaseline)
	s.mux.HandleFunc("POST /api/cases/{caseID}/baseline/precheck", s.HandlePrecheckBaseline)
	s.mux.HandleFunc("POST /api/cases/{caseID}/preflight", s.HandlePreflight)
	s.mux.HandleFunc("POST /api/cases/{caseID}/sessions/start", s.HandleStartSession)
	s.mux.HandleFunc("POST /api/cases/{caseID}/sessions/events", s.HandleRecordEvent)
	s.mux.HandleFunc("POST /api/cases/{caseID}/sessions/observations", s.HandleRecordObservation)
	s.mux.HandleFunc("POST /api/cases/{caseID}/sessions/corrections", s.HandleCorrectSessionRecord)
	s.mux.HandleFunc("POST /api/cases/{caseID}/sessions/finish", s.HandleFinishSession)
	s.mux.HandleFunc("POST /api/cases/{caseID}/deviations/remediate", s.HandleRemediate)
	s.mux.HandleFunc("POST /api/cases/{caseID}/deviations/remediate-batch", s.HandleBatchRemediate)
	s.mux.HandleFunc("POST /api/cases/{caseID}/review", s.HandleReview)
	s.mux.HandleFunc("GET /api/cases/{caseID}/dossier/verify", s.HandleVerifyDossier)
	s.mux.HandleFunc("GET /api/cases/{caseID}/dossier/download", s.HandleDownloadDossier)
}

func (s *Server) Handler() http.Handler {
	return securityHeaders(requestLog(s.mux))
}
