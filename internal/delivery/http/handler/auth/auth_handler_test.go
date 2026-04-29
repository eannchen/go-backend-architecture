package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v5"

	"github.com/eannchen/go-backend-architecture/internal/delivery/http/binding"
	"github.com/eannchen/go-backend-architecture/internal/delivery/http/httpcontext"
	openapi "github.com/eannchen/go-backend-architecture/internal/delivery/http/openapi/gen"
	httpresponse "github.com/eannchen/go-backend-architecture/internal/delivery/http/response"
	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/usecase/auth"
	authoauthtest "github.com/eannchen/go-backend-architecture/internal/usecase/auth/oauth/oauthtest"
	authotptest "github.com/eannchen/go-backend-architecture/internal/usecase/auth/otp/otptest"
	"github.com/eannchen/go-backend-architecture/internal/usecase/auth/session/sessiontest"
)

type echoValidator struct {
	v *validator.Validate
}

func (v *echoValidator) Validate(i any) error {
	return v.v.Struct(i)
}

func newHandlerForTest(otp *authotptest.OTPAuthenticator, session *sessiontest.SessionManager) *Handler {
	return NewHandler(
		logger.NoopLogger{},
		nil,
		httpresponse.NewResponder(nil),
		otp,
		&authoauthtest.OAuthAuthenticator{},
		session,
		SessionCookieConfig{
			Name:   "session_id",
			Secure: false,
			TTL:    30 * time.Minute,
		},
		nil,
	)
}

func newEchoForTest() *echo.Echo {
	e := echo.New()
	e.Binder = binding.NewNormalizeBinder(nil)
	e.Validator = &echoValidator{v: validator.New()}
	return e
}

func TestHandlerVerifyOTPSetsCookieAndReturnsAuthResponse(t *testing.T) {
	otp := &authotptest.OTPAuthenticator{
		VerifyCodeFunc: func(_ context.Context, _, _ string) (auth.Identity, error) {
			return auth.Identity{
				UserID: 99,
				Email:  "user@example.com",
				Method: auth.MethodOTP,
			}, nil
		},
	}
	session := &sessiontest.SessionManager{
		CreateFunc: func(context.Context, auth.Identity) (auth.Session, error) {
			return auth.Session{Token: "session-token"}, nil
		},
	}
	h := newHandlerForTest(otp, session)

	e := newEchoForTest()
	body := `{"email":" USER@EXAMPLE.COM ","code":" ab12 "}`
	req := httptest.NewRequest(http.MethodPost, "/auth/otp/verify", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.VerifyOTP(c); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if otp.VerifyCodeEmail != "user@example.com" {
		t.Fatalf("expected normalized email, got %q", otp.VerifyCodeEmail)
	}
	if otp.VerifyCodeCode != "AB12" {
		t.Fatalf("expected normalized code, got %q", otp.VerifyCodeCode)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected session cookie in response")
	}
	if cookies[0].Name != "session_id" || cookies[0].Value != "session-token" {
		t.Fatalf("unexpected session cookie: %+v", cookies[0])
	}

	var got openapi.AuthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.UserId != 99 || got.Email != "user@example.com" {
		t.Fatalf("unexpected auth response: %+v", got)
	}
}

func TestHandlerLogoutClearsCookieWithoutIncomingSession(t *testing.T) {
	otp := &authotptest.OTPAuthenticator{}
	session := &sessiontest.SessionManager{}
	h := newHandlerForTest(otp, session)

	e := newEchoForTest()
	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.Logout(c); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if session.RevokeCalls != 0 {
		t.Fatalf("expected no revoke calls when no cookie present, got %d", session.RevokeCalls)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected clearing cookie in response")
	}
	if cookies[0].Name != "session_id" || cookies[0].MaxAge != -1 {
		t.Fatalf("expected cleared cookie, got %+v", cookies[0])
	}
}

func TestHandlerMeReturnsSessionFromContext(t *testing.T) {
	otp := &authotptest.OTPAuthenticator{}
	session := &sessiontest.SessionManager{}
	h := newHandlerForTest(otp, session)

	e := newEchoForTest()
	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	httpcontext.SetSessionContext(c, httpcontext.SessionInfo{
		UserID: 10,
		Email:  "me@example.com",
		Method: auth.MethodOTP,
	})

	if err := h.Me(c); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var got openapi.AuthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.UserId != 10 || got.Email != "me@example.com" {
		t.Fatalf("unexpected auth response: %+v", got)
	}
}

func TestHandlerOAuthCallbackInvalidQueryReturnsBadRequest(t *testing.T) {
	otp := &authotptest.OTPAuthenticator{}
	session := &sessiontest.SessionManager{}
	h := newHandlerForTest(otp, session)

	e := newEchoForTest()
	req := httptest.NewRequest(http.MethodGet, "/auth/oauth/google/callback?code=abc", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/auth/oauth/:provider/callback")
	c.SetPathValues(echo.PathValues{{Name: "provider", Value: "google"}})

	if err := h.OAuthCallback(c); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got["code"] != "INVALID_QUERY" {
		t.Fatalf("expected INVALID_QUERY, got %#v", got["code"])
	}
}
