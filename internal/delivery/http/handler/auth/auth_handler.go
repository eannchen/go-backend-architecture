package auth

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v5"

	openapi "github.com/eannchen/go-backend-architecture/internal/delivery/http/openapi/gen"
	httpresponse "github.com/eannchen/go-backend-architecture/internal/delivery/http/response"
	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/observability"
	authoauth "github.com/eannchen/go-backend-architecture/internal/usecase/auth/oauth"
	authotp "github.com/eannchen/go-backend-architecture/internal/usecase/auth/otp"
	authsession "github.com/eannchen/go-backend-architecture/internal/usecase/auth/session"
)

// SessionCookieConfig controls how the session cookie is set in the browser.
type SessionCookieConfig struct {
	Name   string
	Secure bool
	TTL    time.Duration
}

// Handler serves auth-related HTTP endpoints.
type Handler struct {
	logger    logger.Logger
	tracer    observability.Tracer
	responder httpresponse.Responder
	otp       authotp.OTPAuthenticator
	oauth     authoauth.OAuthAuthenticator
	session   authsession.SessionManager
	cookie    SessionCookieConfig
}

func NewHandler(
	log logger.Logger,
	tracer observability.Tracer,
	responder httpresponse.Responder,
	otp authotp.OTPAuthenticator,
	oauth authoauth.OAuthAuthenticator,
	session authsession.SessionManager,
	cookie SessionCookieConfig,
) *Handler {
	if tracer == nil {
		tracer = observability.NoopTracer{}
	}
	if responder == nil {
		responder = httpresponse.NewResponder(nil)
	}
	return &Handler{
		logger:    log,
		tracer:    tracer,
		responder: responder,
		otp:       otp,
		oauth:     oauth,
		session:   session,
		cookie:    cookie,
	}
}

func (h *Handler) RegisterRoutes(e *echo.Echo) {
	e.POST("/auth/otp/send", h.SendOTP)
	e.POST("/auth/otp/verify", h.VerifyOTP)
	e.GET("/auth/oauth/:provider/authorize", h.OAuthAuthorize)
	e.GET("/auth/oauth/:provider/callback", h.OAuthCallback)
	e.POST("/auth/logout", h.Logout)
	e.GET("/auth/me", h.Me)
}

func (h *Handler) SendOTP(c *echo.Context) error {
	ctx, span := h.tracer.Start(c.Request().Context(), "handler", "auth_handler.send_otp")
	var spanErr error
	defer func() { span.Finish(spanErr) }()

	var req otpSendRequest
	if err := c.Bind(&req); err != nil {
		spanErr = err
		return h.responder.InvalidQuery(c, err, "invalid request body")
	}
	if err := c.Validate(&req); err != nil {
		spanErr = err
		return h.responder.InvalidQuery(c, err, "validation failed")
	}

	if err := h.otp.SendCode(ctx, req.Email); err != nil {
		spanErr = err
		return h.responder.AppError(c, err)
	}

	return h.responder.Success(c, http.StatusOK, openapi.MessageResponse{
		Message: "otp code sent",
	})
}

func (h *Handler) VerifyOTP(c *echo.Context) error {
	ctx, span := h.tracer.Start(c.Request().Context(), "handler", "auth_handler.verify_otp")
	var spanErr error
	defer func() { span.Finish(spanErr) }()

	var req otpVerifyRequest
	if err := c.Bind(&req); err != nil {
		spanErr = err
		return h.responder.InvalidQuery(c, err, "invalid request body")
	}
	if err := c.Validate(&req); err != nil {
		spanErr = err
		return h.responder.InvalidQuery(c, err, "validation failed")
	}

	identity, err := h.otp.VerifyCode(ctx, req.Email, req.Code)
	if err != nil {
		spanErr = err
		return h.responder.AppError(c, err)
	}

	session, err := h.session.Create(ctx, identity)
	if err != nil {
		spanErr = err
		return h.responder.AppError(c, err)
	}

	h.setSessionCookie(c, session.Token)

	return h.responder.Success(c, http.StatusOK, openapi.AuthResponse{
		UserId: identity.UserID,
		Email:  identity.Email,
	})
}

func (h *Handler) OAuthAuthorize(c *echo.Context) error {
	ctx, span := h.tracer.Start(c.Request().Context(), "handler", "auth_handler.oauth_authorize")
	var spanErr error
	defer func() { span.Finish(spanErr) }()

	provider := c.Param("provider")

	redirectURL, err := h.oauth.AuthorizeURL(ctx, provider)
	if err != nil {
		spanErr = err
		return h.responder.AppError(c, err)
	}

	return c.Redirect(http.StatusFound, redirectURL)
}

func (h *Handler) OAuthCallback(c *echo.Context) error {
	ctx, span := h.tracer.Start(c.Request().Context(), "handler", "auth_handler.oauth_callback")
	var spanErr error
	defer func() { span.Finish(spanErr) }()

	provider := c.Param("provider")

	var q oauthCallbackQuery
	if err := c.Bind(&q); err != nil {
		spanErr = err
		return h.responder.InvalidQuery(c, err, "invalid callback parameters")
	}
	if err := c.Validate(&q); err != nil {
		spanErr = err
		return h.responder.InvalidQuery(c, err, "validation failed")
	}

	identity, err := h.oauth.HandleCallback(ctx, provider, q.Code, q.State)
	if err != nil {
		spanErr = err
		return h.responder.AppError(c, err)
	}

	session, err := h.session.Create(ctx, identity)
	if err != nil {
		spanErr = err
		return h.responder.AppError(c, err)
	}

	h.setSessionCookie(c, session.Token)

	return h.responder.Success(c, http.StatusOK, openapi.AuthResponse{
		UserId: identity.UserID,
		Email:  identity.Email,
	})
}

func (h *Handler) Logout(c *echo.Context) error {
	ctx, span := h.tracer.Start(c.Request().Context(), "handler", "auth_handler.logout")
	var spanErr error
	defer func() { span.Finish(spanErr) }()

	cookie, err := c.Cookie(h.cookie.Name)
	if err == nil && cookie != nil {
		if revokeErr := h.session.Revoke(ctx, cookie.Value); revokeErr != nil {
			h.logger.Warn(ctx, "session revoke on logout failed", logger.FromPairs("error", revokeErr))
		}
	}

	h.clearSessionCookie(c)

	return h.responder.Success(c, http.StatusOK, openapi.MessageResponse{
		Message: "logged out",
	})
}

func (h *Handler) Me(c *echo.Context) error {
	_, span := h.tracer.Start(c.Request().Context(), "handler", "auth_handler.me")
	var spanErr error
	defer func() { span.Finish(spanErr) }()

	session, ok := SessionFromContext(c)
	if !ok {
		return h.responder.Error(c, nil, httpresponse.Code("UNAUTHORIZED"), "not authenticated")
	}

	return h.responder.Success(c, http.StatusOK, openapi.AuthResponse{
		UserId: session.UserID,
		Email:  session.Email,
	})
}

func (h *Handler) setSessionCookie(c *echo.Context, token string) {
	c.SetCookie(&http.Cookie{
		Name:     h.cookie.Name,
		Value:    token,
		Path:     "/",
		MaxAge:   int(h.cookie.TTL.Seconds()),
		HttpOnly: true,
		Secure:   h.cookie.Secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *Handler) clearSessionCookie(c *echo.Context) {
	c.SetCookie(&http.Cookie{
		Name:     h.cookie.Name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cookie.Secure,
		SameSite: http.SameSiteLaxMode,
	})
}
