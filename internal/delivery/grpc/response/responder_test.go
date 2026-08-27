package response

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
)

func TestResponderMapsApplicationErrors(t *testing.T) {
	tests := []struct {
		name string
		code apperr.Code
		want codes.Code
	}{
		{name: "invalid argument", code: apperr.CodeInvalidArgument, want: codes.InvalidArgument},
		{name: "unauthorized", code: apperr.CodeUnauthorized, want: codes.Unauthenticated},
		{name: "forbidden", code: apperr.CodeForbidden, want: codes.PermissionDenied},
		{name: "not found", code: apperr.CodeNotFound, want: codes.NotFound},
		{name: "conflict", code: apperr.CodeConflict, want: codes.AlreadyExists},
		{name: "too many requests", code: apperr.CodeTooManyRequests, want: codes.ResourceExhausted},
		{name: "unavailable", code: apperr.CodeUnavailable, want: codes.Unavailable},
		{name: "timeout", code: apperr.CodeTimeout, want: codes.DeadlineExceeded},
		{name: "internal", code: apperr.CodeInternal, want: codes.Internal},
	}

	responder := NewResponder()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cause := errors.New("original cause")
			appErr := apperr.Wrap(cause, tt.code, "safe message")

			got := responder.AppError(appErr)

			if status.Code(got) != tt.want {
				t.Fatalf("status code = %v, want %v", status.Code(got), tt.want)
			}
			if message := status.Convert(got).Message(); message != "safe message" {
				t.Fatalf("status message = %q, want safe message", message)
			}
			if !errors.Is(got, cause) {
				t.Fatal("response error does not preserve the original cause")
			}
		})
	}
}

func TestResponderMapsContextErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want codes.Code
	}{
		{name: "canceled", err: context.Canceled, want: codes.Canceled},
		{name: "deadline exceeded", err: context.DeadlineExceeded, want: codes.DeadlineExceeded},
	}

	responder := NewResponder()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := responder.AppError(tt.err)

			if status.Code(got) != tt.want {
				t.Fatalf("status code = %v, want %v", status.Code(got), tt.want)
			}
			if !errors.Is(got, tt.err) {
				t.Fatal("response error does not preserve the context error")
			}
		})
	}
}

func TestResponderHidesUnknownError(t *testing.T) {
	cause := errors.New("database password leaked")

	got := NewResponder().AppError(cause)

	if status.Code(got) != codes.Internal {
		t.Fatalf("status code = %v, want %v", status.Code(got), codes.Internal)
	}
	if message := status.Convert(got).Message(); message != "internal server error" {
		t.Fatalf("status message = %q, want generic message", message)
	}
	if !errors.Is(got, cause) {
		t.Fatal("response error does not preserve the original cause")
	}
}

func TestResponderErrorUsesExplicitTransportStatus(t *testing.T) {
	cause := errors.New("invalid enum")

	got := NewResponder().Error(cause, codes.InvalidArgument, "unsupported health check mode")

	if status.Code(got) != codes.InvalidArgument {
		t.Fatalf("status code = %v, want %v", status.Code(got), codes.InvalidArgument)
	}
	if message := status.Convert(got).Message(); message != "unsupported health check mode" {
		t.Fatalf("status message = %q, want transport message", message)
	}
	if !errors.Is(got, cause) {
		t.Fatal("response error does not preserve the original cause")
	}
}

func TestResponseErrorClassifiesClientStatuses(t *testing.T) {
	tests := []struct {
		code codes.Code
		want bool
	}{
		{code: codes.InvalidArgument, want: true},
		{code: codes.Unauthenticated, want: true},
		{code: codes.Internal, want: false},
		{code: codes.Unavailable, want: false},
	}

	for _, tt := range tests {
		err := NewResponder().Error(errors.New("cause"), tt.code, "safe")
		reporter, ok := err.(interface{ IsClientError() bool })
		if !ok || reporter.IsClientError() != tt.want {
			t.Fatalf("code %v client error = %v, want %v", tt.code, ok && reporter.IsClientError(), tt.want)
		}
	}
}
