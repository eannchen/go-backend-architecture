package response

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
)

// Responder maps delivery and application errors to gRPC statuses.
type Responder interface {
	Error(cause error, code codes.Code, message string) error
	AppError(err error) error
}

type responder struct{}

// NewResponder creates an injectable gRPC responder.
func NewResponder() Responder {
	return &responder{}
}

func (r *responder) Error(cause error, code codes.Code, message string) error {
	return &responseError{
		cause:      cause,
		grpcStatus: status.New(code, message),
	}
}

func (r *responder) AppError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, context.Canceled):
		return r.Error(err, codes.Canceled, context.Canceled.Error())
	case errors.Is(err, context.DeadlineExceeded):
		return r.Error(err, codes.DeadlineExceeded, context.DeadlineExceeded.Error())
	}

	appErr, ok := apperr.As(err)
	if !ok {
		return r.Error(err, codes.Internal, "internal server error")
	}

	code, ok := appCodeToGRPCCode[appErr.Code]
	if !ok {
		return r.Error(err, codes.Internal, "internal server error")
	}
	return r.Error(err, code, appErr.Message)
}

var appCodeToGRPCCode = map[apperr.Code]codes.Code{
	apperr.CodeInvalidArgument: codes.InvalidArgument,
	apperr.CodeUnauthorized:    codes.Unauthenticated,
	apperr.CodeForbidden:       codes.PermissionDenied,
	apperr.CodeNotFound:        codes.NotFound,
	apperr.CodeConflict:        codes.AlreadyExists,
	apperr.CodeTooManyRequests: codes.ResourceExhausted,
	apperr.CodeUnavailable:     codes.Unavailable,
	apperr.CodeTimeout:         codes.DeadlineExceeded,
	apperr.CodeInternal:        codes.Internal,
}

// responseError exposes a safe wire status while retaining the original cause
// for logging and tracing outside the service method.
type responseError struct {
	cause      error
	grpcStatus *status.Status
}

// Error satisfies Go's error interface while exposing only the safe gRPC status text.
func (e *responseError) Error() string {
	return e.grpcStatus.Err().Error()
}

// Unwrap retains the original cause for errors.Is, errors.As, logging, and tracing.
func (e *responseError) Unwrap() error {
	return e.cause
}

// GRPCStatus lets grpc-go serialize the intended status code and safe message.
func (e *responseError) GRPCStatus() *status.Status {
	return e.grpcStatus
}

// IsClientError lets transport-neutral tracing distinguish expected client
// outcomes from server failures without importing gRPC.
func (e *responseError) IsClientError() bool {
	switch e.grpcStatus.Code() {
	case codes.Canceled,
		codes.InvalidArgument,
		codes.NotFound,
		codes.AlreadyExists,
		codes.PermissionDenied,
		codes.Unauthenticated,
		codes.ResourceExhausted,
		codes.FailedPrecondition,
		codes.Aborted,
		codes.OutOfRange:
		return true
	default:
		return false
	}
}
