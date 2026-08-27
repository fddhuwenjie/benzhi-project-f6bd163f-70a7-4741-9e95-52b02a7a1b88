package web

import (
	_ "embed"
	"net/http"
)

//go:embed web/index.html
var indexHTML []byte

//go:embed web/app.css
var appCSS []byte

//go:embed web/app.js
var appJavaScript []byte

func (s *Server) HandleWorkspace(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(indexHTML)
}

func (s *Server) HandleCSS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(appCSS)
}

func (s *Server) HandleJavaScript(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(appJavaScript)
}
