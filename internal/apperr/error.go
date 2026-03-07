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

func New(code Code, message string, details ...Details) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Details: optionalDetails(details...),
	}
}

func Wrap(cause error, code Code, message string, details ...Details) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Details: optionalDetails(details...),
		Cause:   cause,
	}
}

func optionalDetails(details ...Details) Details {
	if len(details) == 0 {
		return nil
	}
	return details[0]
}

func Fields(pairs ...any) Details {
	if len(pairs) == 0 {
		return nil
	}

	details := make(Details, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		key, ok := pairs[i].(string)
		if !ok {
			continue
		}
		details[key] = pairs[i+1]
	}
	if len(details) == 0 {
		return nil
	}
	return details
}

func As(err error) (*Error, bool) {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr, true
	}
	return nil, false
}
