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
	"github.com/eannchen/go-backend-architecture/internal/logger/loggertest"
	"github.com/eannchen/go-backend-architecture/internal/usecase/auth"
	authoauth "github.com/eannchen/go-backend-architecture/internal/usecase/auth/oauth"
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
		&loggertest.Logger{},
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

func TestHandlerOAuthFlowBindsCallbackToAuthorizeBrowser(t *testing.T) {
	oauth := &authoauthtest.OAuthAuthenticator{
		AuthorizeFunc: func(context.Context, string) (authoauth.Authorization, error) {
			return authoauth.Authorization{
				RedirectURL:    "https://provider.example/authorize",
				BrowserBinding: "browser-binding",
			}, nil
		},
		HandleCallbackFunc: func(_ context.Context, provider, code, state, browserBinding string) (auth.Identity, error) {
			if provider != "google" || code != "code" || state != "state" || browserBinding != "browser-binding" {
				t.Fatalf("unexpected callback inputs: provider=%q code=%q state=%q binding=%q", provider, code, state, browserBinding)
			}
			return auth.Identity{UserID: 7, Email: "user@example.com", Method: auth.MethodOAuth}, nil
		},
	}
	session := &sessiontest.SessionManager{
		CreateFunc: func(context.Context, auth.Identity) (auth.Session, error) {
			return auth.Session{Token: "session-token"}, nil
		},
	}
	h := NewHandler(
		&loggertest.Logger{}, nil, httpresponse.NewResponder(nil), &authotptest.OTPAuthenticator{}, oauth, session,
		SessionCookieConfig{Name: "session_id", TTL: 30 * time.Minute}, nil,
	)
	e := newEchoForTest()

	authorizeReq := httptest.NewRequest(http.MethodGet, "/auth/oauth/google/authorize", nil)
	authorizeRec := httptest.NewRecorder()
	authorizeContext := e.NewContext(authorizeReq, authorizeRec)
	authorizeContext.SetPath("/auth/oauth/:provider/authorize")
	authorizeContext.SetPathValues(echo.PathValues{{Name: "provider", Value: "google"}})
	if err := h.OAuthAuthorize(authorizeContext); err != nil {
		t.Fatalf("OAuthAuthorize() error = %v", err)
	}
	if authorizeRec.Code != http.StatusFound {
		t.Fatalf("authorize status = %d, want %d", authorizeRec.Code, http.StatusFound)
	}
	bindingCookie := authorizeRec.Result().Cookies()[0]
	if bindingCookie.Name != oauthBrowserBindingCookieName || bindingCookie.Value != "browser-binding" || !bindingCookie.HttpOnly || bindingCookie.MaxAge != int(authoauth.BrowserBindingTTL.Seconds()) {
		t.Fatalf("unexpected OAuth binding cookie: %+v", bindingCookie)
	}

	callbackReq := httptest.NewRequest(http.MethodGet, "/auth/oauth/google/callback?code=code&state=state", nil)
	callbackReq.AddCookie(bindingCookie)
	callbackRec := httptest.NewRecorder()
	callbackContext := e.NewContext(callbackReq, callbackRec)
	callbackContext.SetPath("/auth/oauth/:provider/callback")
	callbackContext.SetPathValues(echo.PathValues{{Name: "provider", Value: "google"}})
	if err := h.OAuthCallback(callbackContext); err != nil {
		t.Fatalf("OAuthCallback() error = %v", err)
	}
	if callbackRec.Code != http.StatusOK {
		t.Fatalf("callback status = %d, want %d", callbackRec.Code, http.StatusOK)
	}
	callbackCookies := callbackRec.Result().Cookies()
	if len(callbackCookies) != 2 || callbackCookies[0].Name != "session_id" || callbackCookies[1].Name != oauthBrowserBindingCookieName || callbackCookies[1].MaxAge != -1 {
		t.Fatalf("expected session and cleared OAuth binding cookies, got %+v", callbackCookies)
	}
}

func TestHandlerOAuthCallbackRejectsMissingBrowserBinding(t *testing.T) {
	h := newHandlerForTest(&authotptest.OTPAuthenticator{}, &sessiontest.SessionManager{})
	e := newEchoForTest()
	req := httptest.NewRequest(http.MethodGet, "/auth/oauth/google/callback?code=code&state=state", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/auth/oauth/:provider/callback")
	c.SetPathValues(echo.PathValues{{Name: "provider", Value: "google"}})

	if err := h.OAuthCallback(c); err != nil {
		t.Fatalf("OAuthCallback() error = %v", err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("callback status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
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
