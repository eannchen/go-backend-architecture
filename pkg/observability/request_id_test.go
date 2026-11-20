package observability

import (
	"strings"
	"testing"
)

// TestIsValidRequestID defines which request ID shapes are accepted before propagation.
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
