//go:build integration

package integration

import (
	"strings"
	"testing"
)

func TestOTPAuthenticationFlow(t *testing.T) {
	const email = "auth-flow@example.com"

	fixture := newAuthFixture(t, email)

	code := fixture.sendOTP(t, " AUTH-FLOW@EXAMPLE.COM ", email)
	verified, sessionCookie := fixture.verifyOTP(t, " AUTH-FLOW@EXAMPLE.COM ", strings.ToLower(code))
	if verified.UserId == 0 || verified.Email != email {
		t.Fatalf("verify response = %+v, want a user with email %q", verified, email)
	}

	currentUser := fixture.currentUser(t, sessionCookie)
	if currentUser != verified {
		t.Fatalf("current user = %+v, want verified identity %+v", currentUser, verified)
	}

	clearedCookie := fixture.logout(t, sessionCookie)
	if clearedCookie.MaxAge != -1 {
		t.Fatalf("logout cookie max age = %d, want -1", clearedCookie.MaxAge)
	}

	fixture.requireUnauthenticated(t, sessionCookie)
}
