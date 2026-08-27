package observability

import (
	"strings"
	"testing"
)

func TestGenerateRequestID(t *testing.T) {
	first, err := GenerateRequestID()
	if err != nil {
		t.Fatalf("GenerateRequestID() error = %v", err)
	}
	second, err := GenerateRequestID()
	if err != nil {
		t.Fatalf("GenerateRequestID() second error = %v", err)
	}
	if !IsValidRequestID(first) || !IsValidRequestID(second) {
		t.Fatalf("generated IDs are invalid: %q, %q", first, second)
	}
	if first == second {
		t.Fatalf("generated duplicate request ID %q", first)
	}
}

func TestIsValidRequestID(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "letters digits and separators", value: "request_01.trace-ID", want: true},
		{name: "empty", value: "", want: false},
		{name: "too long", value: strings.Repeat("a", MaxRequestIDLength+1), want: false},
		{name: "space", value: "request 01", want: false},
		{name: "non ASCII", value: "request-一", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidRequestID(tt.value); got != tt.want {
				t.Fatalf("IsValidRequestID(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}
