package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/app"
	"github.com/eannchen/go-backend-architecture/internal/logger"
)

func main() {
	os.Exit(run())
}

func run() int {
	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	application, err := app.New(rootCtx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap failed: %v\n", err)
		return 1
	}

	return runLifecycle(rootCtx, stop, lifecycle{
		start:       application.Start,
		shutdown:    application.Shutdown,
		gracePeriod: application.Config.Shutdown.GracePeriod,
		log:         application.Logger,
	})
}

type lifecycle struct {
	start       func() error
	shutdown    func(context.Context) error
	gracePeriod time.Duration
	log         logger.Logger
}

func runLifecycle(rootCtx context.Context, stop context.CancelFunc, application lifecycle) int {
	if application.log == nil {
		application.log = logger.NoopLogger{}
	}

	shutdownDone := make(chan struct{})
	var shutdownErr error
	go func() {
		defer close(shutdownDone)
		<-rootCtx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), application.gracePeriod)
		defer cancel()

		if application.shutdown != nil {
			shutdownErr = application.shutdown(shutdownCtx)
		}
	}()

	exitCode := 0
	if err := application.start(); err != nil {
		if !errors.Is(err, http.ErrServerClosed) {
			application.log.Error(context.Background(), "server exited with error", err, logger.FromPairs("component", "http_server"))
			exitCode = 1
		}
	}

	stop()
	<-shutdownDone
	if shutdownErr != nil {
		application.log.Error(context.Background(), "graceful shutdown failed", shutdownErr, logger.FromPairs("component", "http_server"))
		exitCode = 1
	}

	return exitCode
}
