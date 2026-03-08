package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"go-backend-architecture/internal/app"
	"go-backend-architecture/internal/infra/logger"
)

func main() {
	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	application, err := app.New(rootCtx)
	if err != nil {
		panic(err)
	}

	go func() {
		<-rootCtx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), application.Config.Shutdown.GracePeriod)
		defer cancel()

		if err := application.Shutdown(shutdownCtx); err != nil {
			application.Logger.Error(context.Background(), "graceful shutdown failed", err)
			os.Exit(1)
		}
	}()

	if err := application.Start(); err != nil {
		// Echo returns this error on expected server close during shutdown.
		if !errors.Is(err, http.ErrServerClosed) {
			application.Logger.Error(context.Background(), "server exited with error", err, logger.FromPairs("component", "http_server"))
			os.Exit(1)
		}
	}
}
