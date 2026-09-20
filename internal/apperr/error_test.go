package apperr

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// TestDetailsString checks structured details have a stable string form for reporting.
func TestDetailsString(t *testing.T) {
	tests := []struct {
		name    string
		details Details
		want    string
	}{
		{name: "empty", details: nil, want: ""},
		{name: "JSON", details: Fields("field", "name", "minimum", 3), want: `{"field":"name","minimum":3}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.details.String(); got != tt.want {
				t.Fatalf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestDetailsStringFallsBackForUnsupportedJSONValue checks unsupported detail values still produce a usable string instead of breaking error reporting.
func TestDetailsStringFallsBackForUnsupportedJSONValue(t *testing.T) {
	got := Details{"callback": func() {}}.String()
	if !strings.Contains(got, "callback") {
		t.Fatalf("String() = %q, want fallback containing field name", got)
	}
}

// TestErrorIsClientError checks the client-error classification used by transport responders.
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

// TestErrorFormattingAndWrapping checks formatted errors retain their message, wrapped cause, and matching behavior.
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

// TestConstructorsKeepDetails checks constructors preserve details needed by responders and logs.
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

// TestFields checks error fields expose the expected structured metadata.
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

// TestAs checks wrapped application errors can be recovered through the standard error chain.
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
