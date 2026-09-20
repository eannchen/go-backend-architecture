package user

import "testing"

// TestUserCanAuthenticate checks the user authentication invariant for supported account states.
func TestUserCanAuthenticate(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   bool
	}{
		{name: "active account", status: StatusActive, want: true},
		{name: "disabled account", status: StatusDisabled},
		{name: "unknown account status"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := User{Status: tt.status}
			if got := user.CanAuthenticate(); got != tt.want {
				t.Fatalf("CanAuthenticate() = %t, want %t", got, tt.want)
			}
		})
	}
}
