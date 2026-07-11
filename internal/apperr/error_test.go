package apperr

import "testing"

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
