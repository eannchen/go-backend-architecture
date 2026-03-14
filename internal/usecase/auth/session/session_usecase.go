package session

import (
	"context"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
	"github.com/eannchen/go-backend-architecture/internal/observability"
	repokvstore "github.com/eannchen/go-backend-architecture/internal/repository/kvstore"
	"github.com/eannchen/go-backend-architecture/internal/usecase/auth"
)

// SessionManager manages session lifecycle after authentication.
// Implementations decide how tokens are generated and where state is stored.
//
// Strategy examples:
//   - Server-side session: crypto-random token stored in Redis.
//   - JWT: signed token with claims; Validate decodes without server state.
//   - Access+refresh: dual tokens with separate lifetimes.
type SessionManager interface {
	Create(ctx context.Context, identity auth.Identity) (auth.Session, error)
	Validate(ctx context.Context, token string) (auth.Session, error)
	Revoke(ctx context.Context, token string) error
}

type serverSessionManager struct {
	tracer      observability.Tracer
	sessionRepo repokvstore.SessionRepository
	ttl         time.Duration
	createTotal observability.Counter
}

// NewServerSessionManager creates a server-side session manager.
func NewServerSessionManager(
	tracer observability.Tracer,
	meter observability.Meter,
	sessionRepo repokvstore.SessionRepository,
	ttl time.Duration,
) SessionManager {
	if tracer == nil {
		tracer = observability.NoopTracer{}
	}
	if meter == nil {
		meter = observability.NoopMeter{}
	}
	return &serverSessionManager{
		tracer:      tracer,
		sessionRepo: sessionRepo,
		ttl:         ttl,
		createTotal: meter.Counter("auth_session_create_total",
			observability.MetricOption{Description: "Total sessions created.", Unit: "{session}"},
		),
	}
}

func (m *serverSessionManager) Create(ctx context.Context, identity auth.Identity) (sess auth.Session, err error) {
	ctx, span := m.tracer.Start(ctx, "usecase", "session_manager.create")
	defer func() { span.Finish(err) }()

	token, err := auth.GenerateToken(32)
	if err != nil {
		return auth.Session{}, apperr.Wrap(err, apperr.CodeInternal, "generate session token")
	}

	expiresAt := time.Now().Add(m.ttl)
	data := repokvstore.SessionData{
		Token:     token,
		UserID:    identity.UserID,
		Email:     identity.Email,
		Method:    string(identity.Method),
		ExpiresAt: expiresAt,
	}
	if err := m.sessionRepo.Create(ctx, data, m.ttl); err != nil {
		return auth.Session{}, apperr.Wrap(err, apperr.CodeInternal, "store session")
	}

	m.createTotal.Add(ctx, 1, observability.FromPairs("auth.method", string(identity.Method)))

	return auth.Session{
		Token:     token,
		UserID:    identity.UserID,
		Email:     identity.Email,
		Method:    identity.Method,
		ExpiresAt: expiresAt,
	}, nil
}

func (m *serverSessionManager) Validate(ctx context.Context, token string) (sess auth.Session, err error) {
	ctx, span := m.tracer.Start(ctx, "usecase", "session_manager.validate")
	defer func() { span.Finish(err) }()

	if token == "" {
		return auth.Session{}, apperr.New(apperr.CodeUnauthorized, "missing session token")
	}

	data, err := m.sessionRepo.GetByToken(ctx, token)
	if err != nil {
		return auth.Session{}, apperr.Wrap(err, apperr.CodeUnauthorized, "invalid or expired session")
	}

	if time.Now().After(data.ExpiresAt) {
		_ = m.sessionRepo.Delete(ctx, token)
		return auth.Session{}, apperr.New(apperr.CodeUnauthorized, "session expired")
	}

	return auth.Session{
		Token:     data.Token,
		UserID:    data.UserID,
		Email:     data.Email,
		Method:    auth.MethodType(data.Method),
		ExpiresAt: data.ExpiresAt,
	}, nil
}

func (m *serverSessionManager) Revoke(ctx context.Context, token string) (err error) {
	ctx, span := m.tracer.Start(ctx, "usecase", "session_manager.revoke")
	defer func() { span.Finish(err) }()

	if err := m.sessionRepo.Delete(ctx, token); err != nil {
		return apperr.Wrap(err, apperr.CodeInternal, "revoke session")
	}
	return nil
}
