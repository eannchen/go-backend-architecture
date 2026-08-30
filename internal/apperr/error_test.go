package apperr

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func TestErrorIsClientError(t *testing.T) {
	tests := []struct {
		name string
		code Code
		want bool
	}{
		{name: "invalid argument", code: CodeInvalidArgument, want: true},
		{name: "unauthorized", code: CodeUnauthorized, want: true},
		{name: "forbidden", code: CodeForbidden, want: true},
		{name: "not found", code: CodeNotFound, want: true},
		{name: "conflict", code: CodeConflict, want: true},
		{name: "rate limited", code: CodeTooManyRequests, want: true},
		{name: "unavailable", code: CodeUnavailable, want: false},
		{name: "timeout", code: CodeTimeout, want: false},
		{name: "internal", code: CodeInternal, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := New(tt.code, "test").IsClientError(); got != tt.want {
				t.Fatalf("IsClientError() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestErrorFormattingAndWrapping(t *testing.T) {
	cause := errors.New("database unavailable")
	tests := []struct {
		name       string
		err        *Error
		wantString string
		wantCause  error
	}{
		{
			name:       "new error",
			err:        New(CodeNotFound, "user not found"),
			wantString: "NOT_FOUND: user not found",
		},
		{
			name:       "wrapped error",
			err:        Wrap(cause, CodeUnavailable, "load user"),
			wantString: "UNAVAILABLE: load user: database unavailable",
			wantCause:  cause,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantString {
				t.Fatalf("Error() = %q, want %q", got, tt.wantString)
			}
			if got := errors.Unwrap(tt.err); got != tt.wantCause {
				t.Fatalf("Unwrap() = %v, want %v", got, tt.wantCause)
			}
			if tt.wantCause != nil && !errors.Is(tt.err, tt.wantCause) {
				t.Fatalf("errors.Is() = false, want wrapped cause %v", tt.wantCause)
			}
		})
	}
}

func TestConstructorsKeepDetails(t *testing.T) {
	want := Details{"field": "email"}
	tests := []struct {
		name string
		err  *Error
	}{
		{name: "new", err: New(CodeInvalidArgument, "invalid email", want)},
		{name: "wrap", err: Wrap(errors.New("invalid"), CodeInvalidArgument, "invalid email", want)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !reflect.DeepEqual(tt.err.Details, want) {
				t.Fatalf("Details = %#v, want %#v", tt.err.Details, want)
			}
		})
	}

	if got := New(CodeInternal, "failure").Details; got != nil {
		t.Fatalf("Details without input = %#v, want nil", got)
	}
}

func TestFields(t *testing.T) {
	tests := []struct {
		name  string
		pairs []any
		want  Details
	}{
		{
			name:  "builds details",
			pairs: []any{"field", "email", "retry", 3},
			want:  Details{"field": "email", "retry": 3},
		},
		{
			name:  "ignores non-string and dangling keys",
			pairs: []any{1, "ignored", "kept", true, "dangling"},
			want:  Details{"kept": true},
		},
		{name: "empty returns nil"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Fields(tt.pairs...); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Fields() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestAs(t *testing.T) {
	want := New(CodeConflict, "already exists")
	wrapped := fmt.Errorf("create user: %w", want)

	got, ok := As(wrapped)
	if !ok || got != want {
		t.Fatalf("As() = (%v, %t), want (%v, true)", got, ok, want)
	}
	if got, ok := As(errors.New("plain error")); ok || got != nil {
		t.Fatalf("As(plain error) = (%v, %t), want (nil, false)", got, ok)
	}
}
