package apperr

import (
	"errors"
	"fmt"
)

type Code string

type Details map[string]any

const (
	CodeInvalidArgument Code = "INVALID_ARGUMENT"
	CodeUnauthorized    Code = "UNAUTHORIZED"
	CodeForbidden       Code = "FORBIDDEN"
	CodeNotFound        Code = "NOT_FOUND"
	CodeConflict        Code = "CONFLICT"
	CodeTooManyRequests Code = "TOO_MANY_REQUESTS"
	CodeUnavailable     Code = "UNAVAILABLE"
	CodeTimeout         Code = "TIMEOUT"
	CodeInternal        Code = "INTERNAL"
)

type Error struct {
	Code    Code
	Message string
	Details Details
	Cause   error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error {
	return e.Cause
}

func New(code Code, message string, details Details) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Details: details,
	}
}

func Wrap(cause error, code Code, message string, details Details) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Details: details,
		Cause:   cause,
	}
}

func Field(key string, value any) Details {
	return Details{key: value}
}

func As(err error) (*Error, bool) {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr, true
	}
	return nil, false
}
