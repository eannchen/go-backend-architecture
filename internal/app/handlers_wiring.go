package app

import (
	httpdelivery "github.com/eannchen/go-backend-architecture/internal/delivery/http"
	authhttp "github.com/eannchen/go-backend-architecture/internal/delivery/http/handler/auth"
	healthhttp "github.com/eannchen/go-backend-architecture/internal/delivery/http/handler/health"
	sessionmw "github.com/eannchen/go-backend-architecture/internal/delivery/http/middleware/session"
	httpresponse "github.com/eannchen/go-backend-architecture/internal/delivery/http/response"
)

type appHandlers struct {
	health httpdelivery.RouteRegistrar
	auth   httpdelivery.RouteRegistrar
}

func (d wiring) buildHandlers(responder httpresponse.Responder, usecases appUsecases) appHandlers {
	sessMW := sessionmw.New(usecases.sessionManager, d.cfg.Auth.Session.CookieName, responder)
	return appHandlers{
		health: healthhttp.NewHandler(d.log, d.tracer, responder, usecases.health, healthhttp.StreamConfig{
			CheckInterval:     d.cfg.HTTP.HealthStream.CheckInterval,
			HeartbeatInterval: d.cfg.HTTP.HealthStream.HeartbeatInterval,
			MaxDuration:       d.cfg.HTTP.HealthStream.MaxDuration,
		}),
		auth: authhttp.NewHandler(
			d.log, d.tracer, responder,
			usecases.otpAuth, usecases.oauthAuth, usecases.sessionManager,
			authhttp.SessionCookieConfig{
				Name:   d.cfg.Auth.Session.CookieName,
				Secure: d.cfg.Auth.Session.CookieSecure,
				TTL:    d.cfg.Auth.Session.TTL,
			},
			sessMW,
		),
	}
}
