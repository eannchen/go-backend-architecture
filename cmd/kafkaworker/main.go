package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	kafkaworkerapp "github.com/eannchen/go-backend-architecture/internal/app/kafkaworker"
	appruntime "github.com/eannchen/go-backend-architecture/internal/app/runtime"
)

func main() { os.Exit(run()) }
func run() int {
	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	application, err := kafkaworkerapp.New(rootCtx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap failed: %v\n", err)
		return 1
	}
	return appruntime.RunLifecycle(rootCtx, stop, appruntime.Lifecycle{
		Start:       func() error { return application.Start(rootCtx) },
		Shutdown:    application.Shutdown,
		GracePeriod: application.Config.Shutdown.GracePeriod,
		Logger:      application.Logger,
		Component:   "kafka_worker",
	})
}
