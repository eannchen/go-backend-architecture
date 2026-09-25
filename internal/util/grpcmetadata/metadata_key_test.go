package grpcmetadata

import "testing"

// TestNormalizeKeyNormalizesValidKeys checks valid metadata keys are normalized for reliable matching.
func TestNormalizeKeyNormalizesValidKeys(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want string
	}{
		{name: "disabled", key: "", want: ""},
		{name: "trimmed lowercase", key: "  X-Request-ID  ", want: "x-request-id"},
		{name: "supported separators", key: "request_id.v2", want: "request_id.v2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeKey(tt.key)
			if err != nil {
				t.Fatalf("NormalizeKey() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("NormalizeKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestNormalizeKeyRejectsInvalidKeys checks invalid keys fail before they enter gRPC metadata.
func TestNormalizeKeyRejectsInvalidKeys(t *testing.T) {
	for _, key := range []string{"request id", "request/id", "grpc-timeout"} {
		t.Run(key, func(t *testing.T) {
			if _, err := NormalizeKey(key); err == nil {
				t.Fatalf("NormalizeKey(%q) error = nil", key)
			}
		})
	}
}
