package apperr

import (
	"strings"
	"testing"
)

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

func TestDetailsStringFallsBackForUnsupportedJSONValue(t *testing.T) {
	got := Details{"callback": func() {}}.String()
	if !strings.Contains(got, "callback") {
		t.Fatalf("String() = %q, want fallback containing field name", got)
	}
}

func TestErrorIsClientError(t *testing.T) {
	tests := []struct {
		name string
		code Code
		want bool
	}{
		{name: "invalid argument", code: CodeInvalidArgument, want: true},
		{name: "unauthorized", code: CodeUnauthorized, want: true},
		{name: "not found", code: CodeNotFound, want: true},
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
