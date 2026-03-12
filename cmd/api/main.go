package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/eannchen/go-backend-architecture/internal/app"
	"github.com/eannchen/go-backend-architecture/internal/logger"
)

func main() {
	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	application, err := app.New(rootCtx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap failed: %v\n", err)
		os.Exit(1)
	}

	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-rootCtx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), application.Config.Shutdown.GracePeriod)
		defer cancel()

		if err := application.Shutdown(shutdownCtx); err != nil {
			application.Logger.Error(context.Background(), "graceful shutdown failed", err)
			os.Exit(1)
		}
	}()

	if err := application.Start(); err != nil {
		if !errors.Is(err, http.ErrServerClosed) {
			application.Logger.Error(context.Background(), "server exited with error", err, logger.FromPairs("component", "http_server"))
			os.Exit(1)
		}
	}

	<-shutdownDone
}
