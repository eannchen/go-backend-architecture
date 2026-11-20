package session

import (
	"context"
	"errors"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
	"github.com/eannchen/go-backend-architecture/internal/usecase/auth"
	"github.com/eannchen/go-backend-architecture/pkg/logger"
	"github.com/eannchen/go-backend-architecture/pkg/observability"
)

// Manager implements the session workflow.
type Manager struct {
	log         logger.Logger
	tracer      observability.Tracer
	sessionRepo SessionRepository
	ttl         time.Duration
	createTotal observability.Counter
}

// NewServerSessionManager creates a server-side session manager.
func NewServerSessionManager(
	tracer observability.Tracer,
	log logger.Logger,
	meter observability.Meter,
	sessionRepo SessionRepository,
	ttl time.Duration,
) *Manager {
	return &Manager{
		log:         log,
		tracer:      tracer,
		sessionRepo: sessionRepo,
		ttl:         ttl,
		createTotal: meter.Counter("app.auth.session.creations",
			observability.MetricOption{Description: "Total sessions created.", Unit: "{session}"},
		),
	}
}

func (m *Manager) Create(ctx context.Context, identity auth.Identity) (sess auth.Session, err error) {
	ctx, span := m.tracer.Start(ctx, "usecase", "session_manager.create")
	defer func() { span.Finish(err) }()

	token, err := auth.GenerateToken(32)
	if err != nil {
		return auth.Session{}, apperr.Wrap(err, apperr.CodeInternal, "generate session token")
	}

	expiresAt := time.Now().Add(m.ttl)
	data := SessionData{
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

func (m *Manager) Validate(ctx context.Context, token string) (sess auth.Session, err error) {
	ctx, span := m.tracer.Start(ctx, "usecase", "session_manager.validate")
	defer func() { span.Finish(err) }()

	if token == "" {
		return auth.Session{}, apperr.New(apperr.CodeUnauthorized, "missing session token")
	}

	data, err := m.sessionRepo.GetByToken(ctx, token)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return auth.Session{}, apperr.Wrap(err, apperr.CodeUnauthorized, "invalid or expired session")
		}
		return auth.Session{}, apperr.Wrap(err, apperr.CodeUnavailable, "session store unavailable")
	}

	if time.Now().After(data.ExpiresAt) {
		if deleteErr := m.sessionRepo.Delete(ctx, token); deleteErr != nil {
			m.log.Warn(ctx, "session repository cleanup failed; expired session rejected", logger.FromPairs("error", deleteErr))
		}
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

func (m *Manager) Revoke(ctx context.Context, token string) (err error) {
	ctx, span := m.tracer.Start(ctx, "usecase", "session_manager.revoke")
	defer func() { span.Finish(err) }()

	if err := m.sessionRepo.Delete(ctx, token); err != nil {
		return apperr.Wrap(err, apperr.CodeInternal, "revoke session")
	}
	return nil
}
