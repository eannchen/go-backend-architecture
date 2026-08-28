//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo/v5"

	httpdelivery "github.com/eannchen/go-backend-architecture/internal/delivery/http"
	"github.com/eannchen/go-backend-architecture/internal/delivery/http/binding"
	authhttp "github.com/eannchen/go-backend-architecture/internal/delivery/http/handler/auth"
	sessionmw "github.com/eannchen/go-backend-architecture/internal/delivery/http/middleware/session"
	openapi "github.com/eannchen/go-backend-architecture/internal/delivery/http/openapi/gen"
	httpresponse "github.com/eannchen/go-backend-architecture/internal/delivery/http/response"
	rediscachestore "github.com/eannchen/go-backend-architecture/internal/infra/cache/redis/store"
	composeduser "github.com/eannchen/go-backend-architecture/internal/infra/composed/user"
	postgresstore "github.com/eannchen/go-backend-architecture/internal/infra/db/postgres/store"
	rediskvstore "github.com/eannchen/go-backend-architecture/internal/infra/kvstore/redis/store"
	"github.com/eannchen/go-backend-architecture/internal/logger/loggertest"
	"github.com/eannchen/go-backend-architecture/internal/observability"
	"github.com/eannchen/go-backend-architecture/internal/usecase/auth"
	authoauth "github.com/eannchen/go-backend-architecture/internal/usecase/auth/oauth"
	authotp "github.com/eannchen/go-backend-architecture/internal/usecase/auth/otp"
	authsession "github.com/eannchen/go-backend-architecture/internal/usecase/auth/session"
)

const (
	httpTestSessionCookie = "phase3_session"
	httpTestSessionTTL    = 30 * time.Minute
)

type capturedOTPEmail struct {
	mu    sync.Mutex
	codes map[string]string
}

func newCapturedOTPEmail() *capturedOTPEmail {
	return &capturedOTPEmail{codes: make(map[string]string)}
}

func (s *capturedOTPEmail) SendOTP(_ context.Context, email, code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.codes[email] = code
	return nil
}

func (s *capturedOTPEmail) codeFor(t *testing.T, email string) string {
	t.Helper()

	s.mu.Lock()
	defer s.mu.Unlock()
	code := s.codes[email]
	if code == "" {
		t.Fatalf("no OTP email captured for %q", email)
	}
	return code
}

type unusedOAuthAuthenticator struct{}

func (unusedOAuthAuthenticator) Authorize(context.Context, string) (authoauth.Authorization, error) {
	return authoauth.Authorization{}, fmt.Errorf("OAuth is outside this test flow")
}

func (unusedOAuthAuthenticator) HandleCallback(context.Context, string, string, string, string) (auth.Identity, error) {
	return auth.Identity{}, fmt.Errorf("OAuth is outside this test flow")
}

func TestOTPAuthenticationFlowWithRealPostgresAndRedis(t *testing.T) {
	const email = "phase3@example.com"

	registerHTTPFlowCleanup(t, email)
	server, sentEmail := newRealDependencyAuthServer(t)

	sendJSON(t, server, http.MethodPost, "/auth/otp/send", `{"email":" PHASE3@EXAMPLE.COM "}`, nil, http.StatusOK)
	code := sentEmail.codeFor(t, email)

	verifyBody := fmt.Sprintf(`{"email":" PHASE3@EXAMPLE.COM ","code":" %s "}`, strings.ToLower(code))
	verifyRec := sendJSON(t, server, http.MethodPost, "/auth/otp/verify", verifyBody, nil, http.StatusOK)
	var verified openapi.AuthResponse
	decodeJSON(t, verifyRec, &verified)
	if verified.UserId == 0 || verified.Email != email {
		t.Fatalf("verify response = %+v, want a persisted user with email %q", verified, email)
	}

	assertPersistedUser(t, verified.UserId, email)
	assertRedisKeyMissing(t, "otp:"+email)

	sessionCookie := requireCookie(t, verifyRec, httpTestSessionCookie)
	assertRedisKeyHasTTL(t, "session:"+sessionCookie.Value, httpTestSessionTTL)

	meRec := sendJSON(t, server, http.MethodGet, "/auth/me", "", sessionCookie, http.StatusOK)
	var me openapi.AuthResponse
	decodeJSON(t, meRec, &me)
	if me != verified {
		t.Fatalf("me response = %+v, want verified identity %+v", me, verified)
	}

	logoutRec := sendJSON(t, server, http.MethodPost, "/auth/logout", "", sessionCookie, http.StatusOK)
	cleared := requireCookie(t, logoutRec, httpTestSessionCookie)
	if cleared.MaxAge != -1 {
		t.Fatalf("logout cookie max age = %d, want -1", cleared.MaxAge)
	}
	assertRedisKeyMissing(t, "session:"+sessionCookie.Value)

	sendJSON(t, server, http.MethodGet, "/auth/me", "", sessionCookie, http.StatusUnauthorized)
}

func newRealDependencyAuthServer(t *testing.T) (*httpdelivery.Server, *capturedOTPEmail) {
	t.Helper()

	log := &loggertest.Logger{}
	tracer := observability.NoopTracer{}
	meter := observability.NoopMeter{}

	dbUsers := postgresstore.NewUserStore(httpTestPostgres, tracer)
	userCache := rediscachestore.NewUserCacheStore(httpTestRedis, time.Minute)
	users := composeduser.NewCachedUserStore(log, tracer, dbUsers, userCache)
	otps := rediskvstore.NewOTPStore(httpTestRedis)
	sessions := rediskvstore.NewSessionStore(httpTestRedis)
	sentEmail := newCapturedOTPEmail()
	otpAuthenticator := authotp.NewOTPAuthenticator(log, tracer, meter, otps, sentEmail, users, authotp.OTPConfig{
		CodeLength: 6,
		TTL:        5 * time.Minute,
	})
	sessionManager := authsession.NewServerSessionManager(tracer, meter, sessions, httpTestSessionTTL)
	responder := httpresponse.NewResponder(nil)
	sessionMiddleware := sessionmw.New(sessionManager, httpTestSessionCookie, responder)
	handler := authhttp.NewHandler(
		log,
		tracer,
		responder,
		otpAuthenticator,
		unusedOAuthAuthenticator{},
		sessionManager,
		authhttp.SessionCookieConfig{Name: httpTestSessionCookie, TTL: httpTestSessionTTL},
		sessionMiddleware,
	)

	server, err := httpdelivery.NewServer(
		httpdelivery.ServerConfig{},
		log,
		binding.NewNormalizeBinder(nil),
		nil,
		nil,
		nil,
		handler,
	)
	if err != nil {
		t.Fatalf("build real-dependency HTTP server: %v", err)
	}
	return server, sentEmail
}

func registerHTTPFlowCleanup(t *testing.T, email string) {
	t.Helper()

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		// Session tokens and rate-limit members can be intentionally opaque. Since
		// this package owns its Redis instance, flushing it is explicit per-test cleanup.
		if err := httpTestRedis.FlushDB(ctx).Err(); err != nil {
			t.Errorf("cleanup HTTP-test Redis data: %v", err)
		}
		if _, err := httpTestPostgres.Exec(ctx, "DELETE FROM users WHERE email = $1", email); err != nil {
			t.Errorf("cleanup HTTP-test PostgreSQL user: %v", err)
		}
	})
}

func sendJSON(t *testing.T, server http.Handler, method, path, body string, cookie *http.Cookie, wantStatus int) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
	if cookie != nil {
		request.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != wantStatus {
		t.Fatalf("%s %s status = %d, want %d; body=%s", method, path, response.Code, wantStatus, response.Body.String())
	}
	return response
}

func decodeJSON(t *testing.T, response *httptest.ResponseRecorder, target any) {
	t.Helper()

	if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
		t.Fatalf("decode HTTP response: %v", err)
	}
}

func requireCookie(t *testing.T, response *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()

	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("response did not set cookie %q", name)
	return nil
}

func assertPersistedUser(t *testing.T, id int64, email string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var persistedEmail string
	if err := httpTestPostgres.QueryRow(ctx, "SELECT email FROM users WHERE id = $1", id).Scan(&persistedEmail); err != nil {
		t.Fatalf("query persisted HTTP-test user: %v", err)
	}
	if persistedEmail != email {
		t.Fatalf("persisted email = %q, want %q", persistedEmail, email)
	}
}

func assertRedisKeyMissing(t *testing.T, key string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	exists, err := httpTestRedis.Exists(ctx, key).Result()
	if err != nil {
		t.Fatalf("check Redis key %q: %v", key, err)
	}
	if exists != 0 {
		t.Fatalf("Redis key %q still exists", key)
	}
}

func assertRedisKeyHasTTL(t *testing.T, key string, maximum time.Duration) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ttl, err := httpTestRedis.TTL(ctx, key).Result()
	if err != nil {
		t.Fatalf("read Redis TTL for %q: %v", key, err)
	}
	if ttl <= 0 || ttl > maximum {
		t.Fatalf("Redis TTL for %q = %v, want within (0, %v]", key, ttl, maximum)
	}
}
