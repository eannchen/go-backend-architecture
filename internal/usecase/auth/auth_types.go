package auth

import "time"

// MethodType identifies how a user proved their identity.
type MethodType string

const (
	MethodOTP   MethodType = "otp"
	MethodOAuth MethodType = "oauth"
)

// Identity represents a verified user from any auth method.
type Identity struct {
	UserID int64
	Email  string
	Method MethodType
}

// Session represents an active authenticated session.
type Session struct {
	Token     string
	UserID    int64
	Email     string
	Method    MethodType
	ExpiresAt time.Time
}
