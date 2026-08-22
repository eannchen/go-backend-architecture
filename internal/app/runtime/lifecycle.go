package runtime

import (
	"context"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/logger"
)

type Lifecycle struct {
	Start       func() error
	Shutdown    func(context.Context) error
	GracePeriod time.Duration
	Logger      logger.Logger
	Component   string
}

func RunLifecycle(rootCtx context.Context, stop context.CancelFunc, application Lifecycle) int {
	if application.Logger == nil {
		application.Logger = logger.NoopLogger{}
	}

	shutdownDone := make(chan struct{})
	var shutdownErr error
	go func() {
		defer close(shutdownDone)
		<-rootCtx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), application.GracePeriod)
		defer cancel()

		if application.Shutdown != nil {
			shutdownErr = application.Shutdown(shutdownCtx)
		}
	}()

	exitCode := 0
	if err := application.Start(); err != nil {
		application.Logger.Error(
			context.Background(),
			"server exited with error",
			err,
			logger.FromPairs("component", application.Component),
		)
		exitCode = 1
	}

	stop()
	<-shutdownDone
	if shutdownErr != nil {
		application.Logger.Error(
			context.Background(),
			"graceful shutdown failed",
			shutdownErr,
			logger.FromPairs("component", application.Component),
		)
		exitCode = 1
	}
	return exitCode
}
