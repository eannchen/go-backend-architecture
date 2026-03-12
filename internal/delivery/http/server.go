package http

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/eannchen/go-backend-architecture/internal/logger"
)

// ServerConfig holds HTTP server settings. Filled by app from infra config so delivery does not depend on infra.
type ServerConfig struct {
	Address      string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

type Server struct {
	echo       *echo.Echo
	httpServer *http.Server
	cfg        ServerConfig
	logger     logger.Logger
}

func NewServer(cfg ServerConfig, log logger.Logger, validatorRegistrars []ValidationRegistrar, middlewares []echo.MiddlewareFunc, registrars ...RouteRegistrar) (*Server, error) {
	e := echo.New()
	requestValidator, err := newRequestValidator(validatorRegistrars...)
	if err != nil {
		return nil, fmt.Errorf("initialize request validator: %w", err)
	}
	e.Validator = requestValidator

	for _, m := range middlewares {
		if m == nil {
			continue
		}
		e.Use(m)
	}

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
	}, nil
}

func (s *Server) Start() error {
	s.logger.Info(context.Background(), "http server starting", logger.FromPairs("address", s.cfg.Address))

	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	start := time.Now()
	err := s.httpServer.Shutdown(ctx)
	s.logger.Info(ctx, "http server shutdown complete", logger.FromPairs("duration_ms", time.Since(start).Milliseconds()))
	if err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http server shutdown: %w", err)
	}
	return nil
}
