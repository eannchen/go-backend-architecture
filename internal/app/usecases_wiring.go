package app

import (
	oauthgoogle "github.com/eannchen/go-backend-architecture/internal/infra/external/oauth/login/google"
	otpresend "github.com/eannchen/go-backend-architecture/internal/infra/external/otp/resend"
	otpstub "github.com/eannchen/go-backend-architecture/internal/infra/external/otp/stub"
	repooauth "github.com/eannchen/go-backend-architecture/internal/repository/external/oauth"
	repootp "github.com/eannchen/go-backend-architecture/internal/repository/external/otp"
	authoauth "github.com/eannchen/go-backend-architecture/internal/usecase/auth/oauth"
	authotp "github.com/eannchen/go-backend-architecture/internal/usecase/auth/otp"
	authsession "github.com/eannchen/go-backend-architecture/internal/usecase/auth/session"
	usecasehealth "github.com/eannchen/go-backend-architecture/internal/usecase/health"
)

type appUsecases struct {
	health         usecasehealth.Usecase
	otpAuth        authotp.OTPAuthenticator
	oauthAuth      authoauth.OAuthAuthenticator
	sessionManager authsession.SessionManager
}

func (d wiring) buildOAuthProviders() []repooauth.OAuthProvider {
	var providers []repooauth.OAuthProvider
	gcfg := d.cfg.Auth.OAuth.Google
	if gcfg.ClientID != "" {
		providers = append(providers, oauthgoogle.NewProvider(oauthgoogle.Config{
			ClientID:     gcfg.ClientID,
			ClientSecret: gcfg.ClientSecret,
			RedirectURL:  gcfg.RedirectURL,
		}))
	}
	return providers
}

func (d wiring) buildUsecases(repos appRepositories) appUsecases {
	var emailSender repootp.EmailSender
	if d.cfg.Auth.Resend.APIKey != "" && d.cfg.Auth.Resend.From != "" {
		emailSender = otpresend.NewResendSender(d.cfg.Auth.Resend.APIKey, d.cfg.Auth.Resend.From)
	} else {
		emailSender = otpstub.NewStubSender(d.log)
	}

	return appUsecases{
		health: usecasehealth.New(d.tracer, d.meter, repos.dbHealthRepo, repos.cacheHealthStore, repos.kvHealthStore),
		sessionManager: authsession.NewServerSessionManager(
			d.tracer, d.meter, repos.sessionRepo, d.cfg.Auth.Session.TTL,
		),
		otpAuth: authotp.NewOTPAuthenticator(
			d.log, d.tracer, d.meter,
			repos.otpRepo, emailSender, repos.userRepo,
			authotp.OTPConfig{
				CodeLength: d.cfg.Auth.OTP.CodeLength,
				TTL:        d.cfg.Auth.OTP.TTL,
			},
		),
		oauthAuth: authoauth.NewOAuthAuthenticator(
			d.tracer, d.meter,
			repos.oauthStateRepo, repos.userRepo,
			d.buildOAuthProviders()...,
		),
	}
}
