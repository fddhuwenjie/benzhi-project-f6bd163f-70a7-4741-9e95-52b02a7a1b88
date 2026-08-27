package domain

import "fmt"

const (
	CodeValidation  = "validation_error"
	CodeState       = "invalid_state"
	CodeNotFound    = "not_found"
	CodeConflict    = "revision_conflict"
	CodeForbidden   = "forbidden"
	CodeIntegrity   = "integrity_error"
	CodeIdempotency = "idempotency_conflict"
)

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return e.Message }

func NewError(code, format string, args ...any) error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

func ErrorCode(err error) string {
	if err == nil {
		return ""
	}
	if e, ok := err.(*Error); ok {
		return e.Code
	}
	return "internal_error"
}

func Require(condition bool, code, format string, args ...any) error {
	if condition {
		return nil
	}
	return NewError(code, format, args...)
}
