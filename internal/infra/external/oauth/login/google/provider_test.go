package google

import (
	"strings"
	"testing"
)

func TestParseUserInfo(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{
			name: "verified identity",
			body: `{"id":"google-1","email":"user@example.com","verified_email":true}`,
		},
		{
			name:    "missing provider id",
			body:    `{"email":"user@example.com","verified_email":true}`,
			wantErr: "missing id",
		},
		{
			name:    "unverified email",
			body:    `{"id":"google-1","email":"user@example.com","verified_email":false}`,
			wantErr: "missing verified email",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := parseUserInfo(strings.NewReader(tt.body))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("parseUserInfo() error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseUserInfo() error = %v", err)
			}
			if info.ProviderUserID != "google-1" || info.Email != "user@example.com" {
				t.Fatalf("parseUserInfo() = %+v", info)
			}
		})
	}
}
