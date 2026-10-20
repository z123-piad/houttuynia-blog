package app

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

// Error is a request failure with an HTTP status and a message safe to show.
type Error struct {
	Status  int
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *Error) Unwrap() error { return e.Err }

func Invalid(message string) *Error      { return &Error{Status: 400, Message: message} }
func Unauthorized(message string) *Error { return &Error{Status: 401, Message: message} }
func Forbidden(message string) *Error    { return &Error{Status: 403, Message: message} }
func NotFound(message string) *Error     { return &Error{Status: 404, Message: message} }
func Conflict(message string) *Error     { return &Error{Status: 409, Message: message} }
func NotConfigured(message string) *Error {
	return &Error{Status: 503, Message: message}
}

func UniqueViolation(err error) bool     { return pgCode(err) == "23505" }
func ForeignKeyViolation(err error) bool { return pgCode(err) == "23503" }

func pgCode(err error) string {
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		return pe.Code
	}
	return ""
}
