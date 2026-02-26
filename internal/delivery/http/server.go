package http

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	echoMiddleware "github.com/labstack/echo/v4/middleware"

	"vocynex-api/internal/delivery/middleware"
	"vocynex-api/internal/infra/config"
	"vocynex-api/internal/infra/logger"
)

type Server struct {
	echo   *echo.Echo
	cfg    config.HTTPConfig
	logger logger.Logger
}

func NewServer(cfg config.HTTPConfig, log logger.Logger, healthHandler *HealthHandler) *Server {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.Use(echoMiddleware.Recover())
	e.Use(middleware.ContextPropagation(cfg.ReadTimeout))

	e.GET("/healthz", healthHandler.GetHealth)

	return &Server{
		echo:   e,
		cfg:    cfg,
		logger: log,
	}
}

func (s *Server) Start() error {
	s.echo.Server.ReadTimeout = s.cfg.ReadTimeout
	s.echo.Server.WriteTimeout = s.cfg.WriteTimeout
	s.echo.Server.IdleTimeout = s.cfg.IdleTimeout
	s.echo.Server.ReadHeaderTimeout = s.cfg.ReadTimeout

	s.logger.Info(context.Background(), "http server starting", logger.Field{
		Key:   "address",
		Value: s.cfg.Address,
	})

	return s.echo.Start(s.cfg.Address)
}

func (s *Server) Shutdown(ctx context.Context) error {
	start := time.Now()
	err := s.echo.Shutdown(ctx)
	s.logger.Info(ctx, "http server shutdown complete", logger.Field{
		Key:   "duration_ms",
		Value: time.Since(start).Milliseconds(),
	})
	if err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
