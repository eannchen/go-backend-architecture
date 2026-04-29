package sessionmw

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
	"github.com/eannchen/go-backend-architecture/internal/delivery/http/httpcontext"
	httpresponse "github.com/eannchen/go-backend-architecture/internal/delivery/http/response"
	"github.com/eannchen/go-backend-architecture/internal/usecase/auth"
	"github.com/eannchen/go-backend-architecture/internal/usecase/auth/session/sessiontest"
)

func TestSessionMiddlewareRejectsMissingCredentials(t *testing.T) {
	tests := []struct {
		name   string
		cookie *http.Cookie
	}{
		{name: "missing cookie"},
		{name: "empty cookie", cookie: &http.Cookie{Name: "session_id"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mw := New(&sessiontest.SessionManager{}, "session_id", httpresponse.NewResponder(nil))
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/me", nil)
			if tt.cookie != nil {
				req.AddCookie(tt.cookie)
			}
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			err := mw.Handler()(func(*echo.Context) error {
				t.Fatal("next handler should not be called")
				return nil
			})(c)

			if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d", rec.Code)
			}
			var got map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if got["code"] != string(apperr.CodeUnauthorized) {
				t.Fatalf("expected code %q, got %#v", apperr.CodeUnauthorized, got["code"])
			}
		})
	}
}

func TestSessionMiddlewareReturnsValidationError(t *testing.T) {
	session := &sessiontest.SessionManager{
		ValidateFunc: func(context.Context, string) (auth.Session, error) {
			return auth.Session{}, apperr.New(apperr.CodeUnauthorized, "invalid session")
		},
	}
	mw := New(session, "session_id", httpresponse.NewResponder(nil))
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "bad-token"})
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := mw.Handler()(func(*echo.Context) error {
		t.Fatal("next handler should not be called")
		return nil
	})(c)

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	if session.ValidateCalls != 1 || session.ValidateToken != "bad-token" {
		t.Fatalf("unexpected validate call state: calls=%d token=%q", session.ValidateCalls, session.ValidateToken)
	}
}

func TestSessionMiddlewareSetsSessionContext(t *testing.T) {
	wantSession := auth.Session{
		Token:     "token-1",
		UserID:    42,
		Email:     "user@example.com",
		Method:    auth.MethodOTP,
		ExpiresAt: time.Now().Add(time.Hour),
	}
	session := &sessiontest.SessionManager{
		ValidateFunc: func(context.Context, string) (auth.Session, error) {
			return wantSession, nil
		},
	}
	mw := New(session, "session_id", httpresponse.NewResponder(nil))
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "token-1"})
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	nextCalls := 0

	err := mw.Handler()(func(c *echo.Context) error {
		nextCalls++
		got, ok := httpcontext.SessionFromContext(c)
		if !ok {
			t.Fatal("expected session in context")
		}
		if got.UserID != wantSession.UserID || got.Email != wantSession.Email || got.Method != wantSession.Method {
			t.Fatalf("unexpected session context: %+v", got)
		}
		return c.NoContent(http.StatusNoContent)
	})(c)

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if nextCalls != 1 {
		t.Fatalf("expected next handler once, got %d", nextCalls)
	}
	if session.ValidateCalls != 1 || session.ValidateToken != "token-1" {
		t.Fatalf("unexpected validate call state: calls=%d token=%q", session.ValidateCalls, session.ValidateToken)
	}
}
