package runtime

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/logger"
)

// Worker owns one processing loop and its reader. Shutdown is called once,
// after Run returns, while shared resources such as telemetry remain available.
// Run returns nil on normal cancellation and an error on processing failure.
type Worker struct {
	Name     string
	Run      func(context.Context) error
	Shutdown func(context.Context) error
}

// WorkerGroup coordinates a finite set of workers. StopOnError cancels siblings
// when a worker exits unexpectedly. Otherwise healthy workers keep running.
type WorkerGroup struct {
	Workers         []Worker
	StopOnError     bool
	ShutdownTimeout time.Duration
	Logger          logger.Logger
}

type workerResult struct {
	name string
	err  error
}

// workerErrors preserves causes for errors.Is/As. The outer lifecycle logs
// this summary because each detailed failure is already logged below.
type workerErrors []error

func (e workerErrors) Error() string   { return "workers stopped with errors (see worker logs)" }
func (e workerErrors) Unwrap() []error { return e }

func (g WorkerGroup) Run(ctx context.Context) error {
	if len(g.Workers) == 0 {
		return errors.New("worker group must contain at least one worker")
	}
	if g.ShutdownTimeout <= 0 {
		return errors.New("worker shutdown timeout must be positive")
	}
	for _, worker := range g.Workers {
		if worker.Name == "" || worker.Run == nil || worker.Shutdown == nil {
			return errors.New("worker requires a name, run function, and shutdown function")
		}
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	// One result per worker. Sending never blocks reader cleanup or shutdown.
	results := make(chan workerResult, len(g.Workers))
	for _, worker := range g.Workers {
		go func() {
			err := worker.Run(runCtx)
			if err == nil && runCtx.Err() == nil {
				err = errors.New("worker exited before application cancellation")
			}
			if err != nil && g.StopOnError {
				cancel()
			}
			// Fresh cleanup time is independent of root or sibling cancellation.
			cleanupCtx, stop := context.WithTimeout(context.WithoutCancel(ctx), g.ShutdownTimeout)
			cleanupErr := worker.Shutdown(cleanupCtx)
			stop()
			if cleanupErr != nil {
				err = errors.Join(err, fmt.Errorf("close reader: %w", cleanupErr))
				if g.StopOnError {
					cancel()
				}
			}
			results <- workerResult{name: worker.Name, err: err}
		}()
	}
	var failures workerErrors
	for remaining := len(g.Workers); remaining > 0; remaining-- {
		result := <-results
		if result.err != nil {
			failures = append(failures, fmt.Errorf("worker %s: %w", result.name, result.err))
			g.Logger.Error(ctx, "worker stopped with error", result.err, logger.FromPairs("worker.name", result.name))
		}
	}
	if len(failures) > 0 {
		return failures
	}
	return nil
}
