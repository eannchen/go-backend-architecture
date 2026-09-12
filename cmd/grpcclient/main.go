package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	grpcclientapp "github.com/eannchen/go-backend-architecture/internal/app/grpcclient"
)

func main() {
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	app, err := grpcclientapp.New(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "initialize gRPC client: %v\n", err)
		return 1
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), app.Config.Shutdown.GracePeriod)
		defer cancel()
		if err := app.Shutdown(shutdownCtx); err != nil {
			fmt.Fprintf(os.Stderr, "shutdown gRPC client: %v\n", err)
		}
	}()

	report, err := app.Run(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "run gRPC client demo: %v\n", err)
		return 1
	}
	if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
		fmt.Fprintf(os.Stderr, "write report: %v\n", err)
		return 1
	}
	return 0
}
