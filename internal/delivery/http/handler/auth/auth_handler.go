package auth

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
	"github.com/eannchen/go-backend-architecture/internal/delivery/http/httpcontext"
	sessionmw "github.com/eannchen/go-backend-architecture/internal/delivery/http/middleware/session"
	openapi "github.com/eannchen/go-backend-architecture/internal/delivery/http/openapi/gen"
	httpresponse "github.com/eannchen/go-backend-architecture/internal/delivery/http/response"
	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/observability"
	authoauth "github.com/eannchen/go-backend-architecture/internal/usecase/auth/oauth"
	authotp "github.com/eannchen/go-backend-architecture/internal/usecase/auth/otp"
	authsession "github.com/eannchen/go-backend-architecture/internal/usecase/auth/session"
)

const oauthBrowserBindingCookieName = "oauth_browser_binding"

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
	sessionMW *sessionmw.SessionMiddleware
}

func NewHandler(
	log logger.Logger,
	tracer observability.Tracer,
	responder httpresponse.Responder,
	otp authotp.OTPAuthenticator,
	oauth authoauth.OAuthAuthenticator,
	session authsession.SessionManager,
	cookie SessionCookieConfig,
	sessionMW *sessionmw.SessionMiddleware,
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
		sessionMW: sessionMW,
	}
}

func (h *Handler) RegisterRoutes(e *echo.Echo) {
	e.POST("/auth/otp/send", h.SendOTP)
	e.POST("/auth/otp/verify", h.VerifyOTP)
	e.GET("/auth/oauth/:provider/authorize", h.OAuthAuthorize)
	e.GET("/auth/oauth/:provider/callback", h.OAuthCallback)
	e.POST("/auth/logout", h.Logout)
	e.GET("/auth/me", h.Me, h.sessionMW.Handler())
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

	authorization, err := h.oauth.Authorize(ctx, provider)
	if err != nil {
		spanErr = err
		return h.responder.AppError(c, err)
	}
	h.setOAuthBrowserBindingCookie(c, authorization.BrowserBinding)

	return c.Redirect(http.StatusFound, authorization.RedirectURL)
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

	browserBindingCookie, err := c.Cookie(oauthBrowserBindingCookieName)
	if err != nil || browserBindingCookie == nil || browserBindingCookie.Value == "" {
		spanErr = apperr.New(apperr.CodeUnauthorized, "oauth browser binding is missing or expired")
		return h.responder.AppError(c, spanErr)
	}

	identity, err := h.oauth.HandleCallback(ctx, provider, q.Code, q.State, browserBindingCookie.Value)
	if err != nil {
		spanErr = err
		h.clearOAuthBrowserBindingCookie(c)
		return h.responder.AppError(c, err)
	}

	session, err := h.session.Create(ctx, identity)
	if err != nil {
		spanErr = err
		h.clearOAuthBrowserBindingCookie(c)
		return h.responder.AppError(c, err)
	}

	h.setSessionCookie(c, session.Token)
	// An OAuth state is single-use. Clear its browser proof before writing the
	// response so the Set-Cookie header is not lost after the body is committed.
	h.clearOAuthBrowserBindingCookie(c)

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

// Me returns the current user; requires session middleware (enforced by the route group).
func (h *Handler) Me(c *echo.Context) error {
	_, span := h.tracer.Start(c.Request().Context(), "handler", "auth_handler.me")
	defer span.Finish(nil)

	session, _ := httpcontext.SessionFromContext(c)
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

func (h *Handler) setOAuthBrowserBindingCookie(c *echo.Context, binding string) {
	c.SetCookie(&http.Cookie{
		Name:     oauthBrowserBindingCookieName,
		Value:    binding,
		Path:     "/auth/oauth/",
		MaxAge:   int(authoauth.BrowserBindingTTL.Seconds()),
		HttpOnly: true,
		Secure:   h.cookie.Secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *Handler) clearOAuthBrowserBindingCookie(c *echo.Context) {
	c.SetCookie(&http.Cookie{
		Name:     oauthBrowserBindingCookieName,
		Value:    "",
		Path:     "/auth/oauth/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cookie.Secure,
		SameSite: http.SameSiteLaxMode,
	})
}
