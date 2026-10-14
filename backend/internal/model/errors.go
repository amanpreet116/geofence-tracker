// Package model holds domain types, ID generation and sentinel errors.
package model

import (
	"errors"
	"net"
	"strings"
	"syscall"
)

// Sentinel errors identify the kind of failure; respond.Error maps them to
// HTTP status codes. Prefer NewError for client-facing messages: its message
// survives further wrapping (fmt.Errorf("create vehicle: %w", err)) intact.
var (
	ErrNotFound             = errors.New("not found")
	ErrConflict             = errors.New("conflict")
	ErrRateLimited          = errors.New("rate limit exceeded")
	ErrUnauthorized         = errors.New("unauthorized")
	ErrUnsupportedMediaType = errors.New("unsupported media type")
	ErrPayloadTooLarge      = errors.New("payload too large")
	ErrUnavailable          = errors.New("service temporarily unavailable")
)

// IsUnavailable reports whether err means a dependency (the database) can't
// be reached right now — a 503, not a 500. It recognizes network errors and
// PostgreSQL connection-class / shutdown SQLSTATEs without importing a driver.
func IsUnavailable(err error) bool {
	if errors.Is(err, ErrUnavailable) {
		return true
	}
	if _, ok := errors.AsType[*net.OpError](err); ok {
		return true
	}
	if errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.ECONNRESET) {
		return true
	}
	if pg, ok := errors.AsType[interface {
		error
		SQLState() string
	}](err); ok {
		code := pg.SQLState()
		// 08xxx connection exception; 57P01-03 admin/crash shutdown, cannot connect now.
		return strings.HasPrefix(code, "08") || code == "57P01" || code == "57P02" || code == "57P03"
	}
	return false
}

// ValidationError reports invalid input. Fields maps a JSON field name to
// what is wrong with it and is returned to the client as "details".
type ValidationError struct {
	Message string
	Fields  map[string]string
}

func (e *ValidationError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "validation failed"
}

// NewValidationError builds a *ValidationError; fields may be nil.
func NewValidationError(message string, fields map[string]string) *ValidationError {
	return &ValidationError{Message: message, Fields: fields}
}

// Error is a domain error with a client-facing message. Kind is one of the
// sentinels above and is matched by errors.Is.
type Error struct {
	Kind    error
	Message string
	Details any // optional; returned to the client as "details"
}

func (e *Error) Error() string { return e.Message }
func (e *Error) Unwrap() error { return e.Kind }

// NewError returns an *Error of the given kind.
func NewError(kind error, message string) *Error {
	return &Error{Kind: kind, Message: message}
}
