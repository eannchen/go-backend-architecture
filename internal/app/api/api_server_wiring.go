package api

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"
	echoMiddleware "github.com/labstack/echo/v5/middleware"

	httpdelivery "github.com/eannchen/go-backend-architecture/internal/delivery/http"
	"github.com/eannchen/go-backend-architecture/internal/delivery/http/binding"
	healthhttp "github.com/eannchen/go-backend-architecture/internal/delivery/http/handler/health"
	bodylimitmw "github.com/eannchen/go-backend-architecture/internal/delivery/http/middleware/bodylimit"
	contextmw "github.com/eannchen/go-backend-architecture/internal/delivery/http/middleware/context"
	observabilitymw "github.com/eannchen/go-backend-architecture/internal/delivery/http/middleware/observability"
	ratelimitmw "github.com/eannchen/go-backend-architecture/internal/delivery/http/middleware/ratelimit"
	httpresponse "github.com/eannchen/go-backend-architecture/internal/delivery/http/response"
	"github.com/eannchen/go-backend-architecture/internal/usecase/globalratelimit"
)

func (d wiring) buildServer(responder httpresponse.Responder, repos appRepositories, handlers appHandlers, usecases appUsecases) (*httpdelivery.Server, error) {
	validatorRegistrars := []httpdelivery.ValidationRegistrar{
		healthhttp.RegisterValidation,
	}

	secureCfg := echoMiddleware.SecureConfig{
		XSSProtection:      "1; mode=block",
		ContentTypeNosniff: "nosniff",
		XFrameOptions:      "DENY",
		ReferrerPolicy:     "strict-origin-when-cross-origin",
	}
	if !isLocalAppEnv(d.cfg.AppEnv) {
		secureCfg.HSTSMaxAge = 31536000
		secureCfg.HSTSPreloadEnabled = true
	}
	globalLimiter := ratelimitmw.NewGlobalRateLimit(globalratelimit.NewIPLimiter(repos.tokenBucketRepo, d.log, globalratelimit.Config{
		Capacity:       d.cfg.RateLimit.GlobalIPCapacity,
		RefillInterval: d.cfg.RateLimit.GlobalIPRefillInterval,
	}), responder, d.meter)
	preMiddlewares := []echo.MiddlewareFunc{
		bodylimitmw.New(d.cfg.HTTP.MaxRequestBodyBytes).Handler(),
	}

	middlewares := []echo.MiddlewareFunc{
		observabilitymw.New(d.tracer, d.log, d.meter).Handler(),
		echoMiddleware.Recover(),
		echoMiddleware.SecureWithConfig(secureCfg),
		globalLimiter.Handler(),
		echoMiddleware.CORSWithConfig(echoMiddleware.CORSConfig{
			AllowOrigins: d.cfg.HTTP.CORSAllowOrigins,
			AllowMethods: []string{
				http.MethodGet,
				http.MethodPost,
				http.MethodPut,
				http.MethodPatch,
				http.MethodDelete,
				http.MethodOptions,
			},
			AllowHeaders: []string{
				echo.HeaderOrigin,
				echo.HeaderContentType,
				echo.HeaderAccept,
				echo.HeaderAuthorization,
			},
			AllowCredentials: true,
		}),
		contextmw.NewRequestContextMiddleware(
			d.cfg.HTTP.RequestTimeout,
			responder,
			contextmw.WithTimeoutSkipper(func(c *echo.Context) bool {
				return c.Request().URL.Path == healthhttp.StreamPath
			}),
		).Handler(),
	}
	ipExtractor, err := buildIPExtractor(d.cfg.HTTP.TrustedProxyCIDRs)
	if err != nil {
		return nil, fmt.Errorf("build ip extractor: %w", err)
	}
	serverCfg := httpdelivery.ServerConfig{
		Address:        d.cfg.HTTP.Address,
		ReadTimeout:    d.cfg.HTTP.ReadTimeout,
		WriteTimeout:   d.cfg.HTTP.WriteTimeout,
		IdleTimeout:    d.cfg.HTTP.IdleTimeout,
		MaxHeaderBytes: d.cfg.HTTP.MaxHeaderBytes,
		IPExtractor:    ipExtractor,
	}
	binder := binding.NewNormalizeBinder(nil)
	return httpdelivery.NewServer(serverCfg, d.log, binder, validatorRegistrars, preMiddlewares, middlewares, handlers.health, handlers.auth)
}

func isLocalAppEnv(env string) bool {
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "local", "dev", "development", "test":
		return true
	default:
		return false
	}
}

func buildIPExtractor(trustedCIDRs []string) (echo.IPExtractor, error) {
	if len(trustedCIDRs) == 0 {
		return echo.ExtractIPDirect(), nil
	}

	opts := []echo.TrustOption{
		echo.TrustLoopback(false),
		echo.TrustLinkLocal(false),
		echo.TrustPrivateNet(false),
	}
	for _, raw := range trustedCIDRs {
		_, ipNet, err := net.ParseCIDR(raw)
		if err != nil {
			return nil, fmt.Errorf("parse trusted proxy cidr %q: %w", raw, err)
		}
		opts = append(opts, echo.TrustIPRange(ipNet))
	}
	return echo.ExtractIPFromXFFHeader(opts...), nil
}
