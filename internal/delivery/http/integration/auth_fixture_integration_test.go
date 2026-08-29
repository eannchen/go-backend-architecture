//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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
	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/observability"
	emailtest "github.com/eannchen/go-backend-architecture/internal/repository/external/otp/otptest"
	"github.com/eannchen/go-backend-architecture/internal/usecase/auth"
	authoauth "github.com/eannchen/go-backend-architecture/internal/usecase/auth/oauth"
	oauthauthtest "github.com/eannchen/go-backend-architecture/internal/usecase/auth/oauth/oauthtest"
	authotp "github.com/eannchen/go-backend-architecture/internal/usecase/auth/otp"
	authsession "github.com/eannchen/go-backend-architecture/internal/usecase/auth/session"
)

const (
	integrationSessionCookie = "integration_session"
	integrationSessionTTL    = 30 * time.Minute
)

var errUnexpectedOAuth = errors.New("OAuth is outside this test flow")

type authFixture struct {
	server      http.Handler
	emailSender *emailtest.EmailSender
}

func newAuthFixture(t *testing.T, email string) *authFixture {
	t.Helper()

	registerAuthCleanup(t, email)

	log := logger.NoopLogger{}
	tracer := observability.NoopTracer{}
	meter := observability.NoopMeter{}
	dbUsers := postgresstore.NewUserStore(postgresPool, tracer)
	userCache := rediscachestore.NewUserCacheStore(redisClient, time.Minute)
	users := composeduser.NewCachedUserStore(log, tracer, dbUsers, userCache)
	otps := rediskvstore.NewOTPStore(redisClient)
	sessions := rediskvstore.NewSessionStore(redisClient)
	emailSender := &emailtest.EmailSender{
		SendOTPFunc: func(context.Context, string, string) error { return nil },
	}
	otpAuthenticator := authotp.NewOTPAuthenticator(log, tracer, meter, otps, emailSender, users, authotp.OTPConfig{
		CodeLength: 6,
		TTL:        5 * time.Minute,
	})
	sessionManager := authsession.NewServerSessionManager(tracer, meter, sessions, integrationSessionTTL)
	responder := httpresponse.NewResponder(nil)
	sessionMiddleware := sessionmw.New(sessionManager, integrationSessionCookie, responder)
	handler := authhttp.NewHandler(
		log,
		tracer,
		responder,
		otpAuthenticator,
		&oauthauthtest.OAuthAuthenticator{
			AuthorizeFunc: func(context.Context, string) (authoauth.Authorization, error) {
				return authoauth.Authorization{}, errUnexpectedOAuth
			},
			HandleCallbackFunc: func(context.Context, string, string, string, string) (auth.Identity, error) {
				return auth.Identity{}, errUnexpectedOAuth
			},
		},
		sessionManager,
		authhttp.SessionCookieConfig{Name: integrationSessionCookie, TTL: integrationSessionTTL},
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
		t.Fatalf("build HTTP integration server: %v", err)
	}
	return &authFixture{server: server, emailSender: emailSender}
}

func (f *authFixture) sendOTP(t *testing.T, inputEmail, deliveredTo string) string {
	t.Helper()

	f.sendJSON(t, http.MethodPost, "/auth/otp/send", map[string]string{"email": inputEmail}, nil, http.StatusOK)
	if f.emailSender.SendOTPCalls != 1 {
		t.Fatalf("email send calls = %d, want 1", f.emailSender.SendOTPCalls)
	}
	if f.emailSender.SendOTPEmail != deliveredTo {
		t.Fatalf("OTP delivered to %q, want %q", f.emailSender.SendOTPEmail, deliveredTo)
	}
	if f.emailSender.SendOTPCode == "" {
		t.Fatal("OTP email contained an empty code")
	}
	return f.emailSender.SendOTPCode
}

func (f *authFixture) verifyOTP(t *testing.T, email, code string) (openapi.AuthResponse, *http.Cookie) {
	t.Helper()

	response := f.sendJSON(t, http.MethodPost, "/auth/otp/verify", map[string]string{
		"email": email,
		"code":  code,
	}, nil, http.StatusOK)
	return decodeAuthResponse(t, response), requireCookie(t, response, integrationSessionCookie)
}

func (f *authFixture) currentUser(t *testing.T, sessionCookie *http.Cookie) openapi.AuthResponse {
	t.Helper()

	response := f.sendJSON(t, http.MethodGet, "/auth/me", nil, sessionCookie, http.StatusOK)
	return decodeAuthResponse(t, response)
}

func (f *authFixture) logout(t *testing.T, sessionCookie *http.Cookie) *http.Cookie {
	t.Helper()

	response := f.sendJSON(t, http.MethodPost, "/auth/logout", nil, sessionCookie, http.StatusOK)
	return requireCookie(t, response, integrationSessionCookie)
}

func (f *authFixture) requireUnauthenticated(t *testing.T, sessionCookie *http.Cookie) {
	t.Helper()

	f.sendJSON(t, http.MethodGet, "/auth/me", nil, sessionCookie, http.StatusUnauthorized)
}

func (f *authFixture) sendJSON(
	t *testing.T,
	method string,
	path string,
	body any,
	cookie *http.Cookie,
	wantStatus int,
) *httptest.ResponseRecorder {
	t.Helper()

	var requestBody []byte
	if body != nil {
		var err error
		requestBody, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("encode %s %s request: %v", method, path, err)
		}
	}

	request := httptest.NewRequest(method, path, bytes.NewReader(requestBody))
	if body != nil {
		request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
	if cookie != nil {
		request.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	f.server.ServeHTTP(response, request)
	if response.Code != wantStatus {
		t.Fatalf("%s %s status = %d, want %d; body=%s", method, path, response.Code, wantStatus, response.Body.String())
	}
	return response
}

func decodeAuthResponse(t *testing.T, response *httptest.ResponseRecorder) openapi.AuthResponse {
	t.Helper()

	var decoded openapi.AuthResponse
	if err := json.Unmarshal(response.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode auth response: %v", err)
	}
	return decoded
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

func registerAuthCleanup(t *testing.T, email string) {
	t.Helper()

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		// This package owns its Redis container, so flushing avoids coupling cleanup
		// to internal key formats such as opaque session tokens.
		if err := redisClient.FlushDB(ctx).Err(); err != nil {
			t.Errorf("cleanup HTTP-test Redis data: %v", err)
		}
		if _, err := postgresPool.Exec(ctx, "DELETE FROM users WHERE email = $1", email); err != nil {
			t.Errorf("cleanup HTTP-test PostgreSQL user: %v", err)
		}
	})
}
