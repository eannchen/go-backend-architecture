package http

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
	echoMiddleware "github.com/labstack/echo/v5/middleware"

	"vocynex-api/internal/delivery/middleware"
	"vocynex-api/internal/infra/config"
	"vocynex-api/internal/infra/logger"
	"vocynex-api/internal/infra/observability"
)

type Server struct {
	echo       *echo.Echo
	httpServer *http.Server
	cfg        config.HTTPConfig
	logger     logger.Logger
}

func NewServer(cfg config.HTTPConfig, log logger.Logger, tracer observability.Tracer, registrars ...RouteRegistrar) *Server {
	e := echo.New()
	e.Validator = newRequestValidator()

	e.Use(echoMiddleware.Recover())
	e.Use(middleware.ContextPropagation(cfg.ReadTimeout))
	e.Use(middleware.Tracing(tracer))

	for _, registrar := range registrars {
		if registrar == nil {
			continue
		}
		registrar.RegisterRoutes(e)
	}

	httpServer := &http.Server{
		Addr:              cfg.Address,
		Handler:           e,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		ReadHeaderTimeout: cfg.ReadTimeout,
	}

	return &Server{
		echo:       e,
		httpServer: httpServer,
		cfg:        cfg,
		logger:     log,
	}
}

func (s *Server) Start() error {
	s.logger.Info(context.Background(), "http server starting", logger.Fields("address", s.cfg.Address))

	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	start := time.Now()
	err := s.httpServer.Shutdown(ctx)
	s.logger.Info(ctx, "http server shutdown complete", logger.Fields("duration_ms", time.Since(start).Milliseconds()))
	if err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
