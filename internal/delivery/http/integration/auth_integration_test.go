package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v5"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
	"github.com/eannchen/go-backend-architecture/internal/delivery/http/binding"
	authhttp "github.com/eannchen/go-backend-architecture/internal/delivery/http/handler/auth"
	sessionmw "github.com/eannchen/go-backend-architecture/internal/delivery/http/middleware/session"
	openapi "github.com/eannchen/go-backend-architecture/internal/delivery/http/openapi/gen"
	httpresponse "github.com/eannchen/go-backend-architecture/internal/delivery/http/response"
	"github.com/eannchen/go-backend-architecture/internal/logger/loggertest"
	"github.com/eannchen/go-backend-architecture/internal/usecase/auth"
)

type inMemoryOTP struct{}

func (inMemoryOTP) SendCode(context.Context, string) error { return nil }

func (inMemoryOTP) VerifyCode(_ context.Context, email, _ string) (auth.Identity, error) {
	return auth.Identity{
		UserID: 1001,
		Email:  email,
		Method: auth.MethodOTP,
	}, nil
}

type inMemoryOAuth struct{}

func (inMemoryOAuth) AuthorizeURL(context.Context, string) (string, error) { return "", nil }
func (inMemoryOAuth) HandleCallback(context.Context, string, string, string) (auth.Identity, error) {
	return auth.Identity{}, nil
}

type inMemorySessionManager struct {
	mu       sync.RWMutex
	nextID   int64
	sessions map[string]auth.Session
}

func newInMemorySessionManager() *inMemorySessionManager {
	return &inMemorySessionManager{
		sessions: make(map[string]auth.Session),
	}
}

func (m *inMemorySessionManager) Create(_ context.Context, identity auth.Identity) (auth.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.nextID++
	token := fmt.Sprintf("token-%d", m.nextID)
	sess := auth.Session{
		Token:     token,
		UserID:    identity.UserID,
		Email:     identity.Email,
		Method:    identity.Method,
		ExpiresAt: time.Now().Add(30 * time.Minute),
	}
	m.sessions[token] = sess
	return sess, nil
}

func (m *inMemorySessionManager) Validate(_ context.Context, token string) (auth.Session, error) {
	if token == "" {
		return auth.Session{}, apperr.New(apperr.CodeUnauthorized, "missing session token")
	}

	m.mu.RLock()
	sess, ok := m.sessions[token]
	m.mu.RUnlock()
	if !ok {
		return auth.Session{}, apperr.New(apperr.CodeUnauthorized, "invalid session")
	}
	if time.Now().After(sess.ExpiresAt) {
		return auth.Session{}, apperr.New(apperr.CodeUnauthorized, "expired session")
	}
	return sess, nil
}

func (m *inMemorySessionManager) Revoke(_ context.Context, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, token)
	return nil
}

type echoValidator struct {
	v *validator.Validate
}

func (v *echoValidator) Validate(i any) error {
	return v.v.Struct(i)
}

func newAuthTestServer() *echo.Echo {
	e := echo.New()
	e.Binder = binding.NewNormalizeBinder(nil)
	e.Validator = &echoValidator{v: validator.New()}

	responder := httpresponse.NewResponder(nil)
	session := newInMemorySessionManager()
	authHandler := authhttp.NewHandler(
		&loggertest.Logger{},
		nil,
		responder,
		inMemoryOTP{},
		inMemoryOAuth{},
		session,
		authhttp.SessionCookieConfig{
			Name:   "session_id",
			Secure: false,
			TTL:    30 * time.Minute,
		},
		sessionmw.New(session, "session_id", responder),
	)
	authHandler.RegisterRoutes(e)

	return e
}

func TestAuthIntegrationVerifyThenMe(t *testing.T) {
	server := newAuthTestServer()

	verifyBody := `{"email":" TEST@EXAMPLE.COM ","code":" a1b2 "}`
	verifyReq := httptest.NewRequest(http.MethodPost, "/auth/otp/verify", bytes.NewBufferString(verifyBody))
	verifyReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	verifyRec := httptest.NewRecorder()
	server.ServeHTTP(verifyRec, verifyReq)

	if verifyRec.Code != http.StatusOK {
		t.Fatalf("expected verify status 200, got %d", verifyRec.Code)
	}

	var verifyResp openapi.AuthResponse
	if err := json.Unmarshal(verifyRec.Body.Bytes(), &verifyResp); err != nil {
		t.Fatalf("decode verify response: %v", err)
	}
	if verifyResp.UserId != 1001 || verifyResp.Email != "test@example.com" {
		t.Fatalf("unexpected verify payload: %+v", verifyResp)
	}

	cookies := verifyRec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected session cookie from verify endpoint")
	}

	meReq := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	meReq.AddCookie(cookies[0])
	meRec := httptest.NewRecorder()
	server.ServeHTTP(meRec, meReq)

	if meRec.Code != http.StatusOK {
		t.Fatalf("expected me status 200, got %d", meRec.Code)
	}

	var meResp openapi.AuthResponse
	if err := json.Unmarshal(meRec.Body.Bytes(), &meResp); err != nil {
		t.Fatalf("decode me response: %v", err)
	}
	if meResp.UserId != 1001 || meResp.Email != "test@example.com" {
		t.Fatalf("unexpected me payload: %+v", meResp)
	}
}

func TestAuthIntegrationLogoutRevokesSession(t *testing.T) {
	server := newAuthTestServer()

	verifyReq := httptest.NewRequest(http.MethodPost, "/auth/otp/verify", bytes.NewBufferString(`{"email":"user@example.com","code":"A1B2"}`))
	verifyReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	verifyRec := httptest.NewRecorder()
	server.ServeHTTP(verifyRec, verifyReq)
	if verifyRec.Code != http.StatusOK {
		t.Fatalf("expected verify status 200, got %d", verifyRec.Code)
	}

	cookies := verifyRec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected session cookie from verify endpoint")
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	logoutReq.AddCookie(cookies[0])
	logoutRec := httptest.NewRecorder()
	server.ServeHTTP(logoutRec, logoutReq)

	if logoutRec.Code != http.StatusOK {
		t.Fatalf("expected logout status 200, got %d", logoutRec.Code)
	}
	logoutCookies := logoutRec.Result().Cookies()
	if len(logoutCookies) == 0 || logoutCookies[0].MaxAge != -1 {
		t.Fatalf("expected clearing cookie with max age -1, got %+v", logoutCookies)
	}

	meReq := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	meReq.AddCookie(cookies[0])
	meRec := httptest.NewRecorder()
	server.ServeHTTP(meRec, meReq)

	if meRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected me status 401 after logout, got %d", meRec.Code)
	}
}
