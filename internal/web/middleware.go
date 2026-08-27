package web

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime"
	"net/http"
	"strings"
	"time"

	"benzhi-project-f6bd163f-70a7-4741-9e95-52b02a7a1b88/internal/application"
	"benzhi-project-f6bd163f-70a7-4741-9e95-52b02a7a1b88/internal/domain"
)

const maxRequestBody = 1 << 20

type apiEnvelope struct {
	Data  any           `json:"data,omitempty"`
	Error *domain.Error `json:"error,omitempty"`
	Meta  *responseMeta `json:"meta,omitempty"`
}

type responseMeta struct {
	Replayed bool `json:"replayed"`
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self' data:; base-uri 'none'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("http method=%s path=%s duration=%s", r.Method, r.URL.Path, time.Since(started).Round(time.Millisecond))
	})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, domain.CodeValidation, "Content-Type 必须为 application/json")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, domain.CodeValidation, "请求体超过 1 MiB 上限")
		} else {
			writeError(w, http.StatusBadRequest, domain.CodeValidation, "JSON 请求无效: "+err.Error())
		}
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeError(w, http.StatusBadRequest, domain.CodeValidation, "请求体只能包含一个 JSON 对象")
		return false
	}
	return true
}

func writeOutcome(w http.ResponseWriter, outcome *application.Outcome, err error) {
	if err != nil {
		log.Printf("application error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "服务内部错误")
		return
	}
	if outcome == nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "应用结果为空")
		return
	}
	envelope := apiEnvelope{Meta: &responseMeta{Replayed: outcome.Replayed}}
	if outcome.Error != nil {
		envelope.Error = outcome.Error
	} else {
		envelope.Data = outcome.Case
	}
	writeJSON(w, outcome.HTTPStatus, envelope)
}

func writeDomainError(w http.ResponseWriter, err error) {
	if e, ok := err.(*domain.Error); ok {
		status := http.StatusInternalServerError
		switch e.Code {
		case domain.CodeValidation, domain.CodeState:
			status = http.StatusUnprocessableEntity
		case domain.CodeForbidden:
			status = http.StatusForbidden
		case domain.CodeNotFound:
			status = http.StatusNotFound
		case domain.CodeConflict, domain.CodeIdempotency:
			status = http.StatusConflict
		case domain.CodeIntegrity:
			status = http.StatusServiceUnavailable
		}
		writeJSON(w, status, apiEnvelope{Error: e})
		return
	}
	log.Printf("query error: %v", err)
	writeError(w, http.StatusInternalServerError, "internal_error", "服务内部错误")
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, apiEnvelope{Error: &domain.Error{Code: code, Message: message}})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil && !strings.Contains(err.Error(), "broken pipe") {
		log.Printf("encode response: %v", err)
	}
}
