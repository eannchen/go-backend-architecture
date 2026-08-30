//go:build integration

// Package redistest owns disposable Redis infrastructure for integration tests.
package redistest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Pin the exact Redis version so a moving image tag cannot change test behavior
// between a developer laptop and CI without a dependency update in this repository.
const redisTestImage = "redis:8.10.0-alpine"

// Instance is one disposable Redis container owned by a test package.
type Instance struct {
	container testcontainers.Container
	client    *goredis.Client
}

// RunPackage owns the complete Redis lifecycle around one package's tests.
func RunPackage(m *testing.M, packageName string, setClient func(*goredis.Client)) int {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	testRedis, err := Start(ctx)
	if err != nil {
		// The integration build tag is an explicit request to exercise Redis. If
		// Docker is unavailable, failing prevents CI from reporting a false success.
		fmt.Fprintf(os.Stderr, "start %s Redis integration dependency: %v\n", packageName, err)
		return 1
	}
	setClient(testRedis.Client())

	exitCode := m.Run()
	exitCode = verifyEmpty(testRedis.Client(), packageName, exitCode)
	if exitCode != 0 {
		fmt.Fprintf(os.Stderr, "--- %s Redis container logs ---\n", packageName)
		if err := testRedis.WriteLogs(os.Stderr); err != nil {
			fmt.Fprintf(os.Stderr, "write %s Redis container logs: %v\n", packageName, err)
		}
	}
	if err := testRedis.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "close %s Redis integration dependency: %v\n", packageName, err)
		if exitCode == 0 {
			exitCode = 1
		}
	}
	return exitCode
}

// Start creates a fresh Redis instance and waits until it accepts commands.
func Start(ctx context.Context) (*Instance, error) {
	container, err := testcontainers.Run(
		ctx,
		redisTestImage,
		testcontainers.WithExposedPorts("6379/tcp"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("Ready to accept connections").
				WithOccurrence(1).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("start Redis container: %w", err)
	}

	endpoint, err := container.Endpoint(ctx, "")
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("get Redis container endpoint: %w", err),
			testcontainers.TerminateContainer(container),
		)
	}

	client := goredis.NewClient(&goredis.Options{Addr: endpoint})
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, errors.Join(
			fmt.Errorf("ping Redis container: %w", err),
			client.Close(),
			testcontainers.TerminateContainer(container),
		)
	}

	return &Instance{container: container, client: client}, nil
}

// Client returns the package-shared client; individual tests must not close it.
func (r *Instance) Client() *goredis.Client {
	return r.client
}

// WriteLogs copies container output for failure diagnostics before cleanup.
func (r *Instance) WriteLogs(writer io.Writer) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	logs, err := r.container.Logs(ctx)
	if err != nil {
		return fmt.Errorf("read Redis container logs: %w", err)
	}
	_, copyErr := io.Copy(writer, logs)
	closeErr := logs.Close()
	if copyErr != nil {
		copyErr = fmt.Errorf("copy Redis container logs: %w", copyErr)
	}
	if closeErr != nil {
		closeErr = fmt.Errorf("close Redis container logs: %w", closeErr)
	}
	return errors.Join(copyErr, closeErr)
}

// Close closes the client and explicitly removes the package container.
func (r *Instance) Close() error {
	// Testcontainers has a resource reaper as a safety net, but explicit cleanup
	// makes ownership deterministic and leaves no service running after the package.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return errors.Join(
		r.client.Close(),
		testcontainers.TerminateContainer(r.container, testcontainers.StopContext(ctx)),
	)
}

func verifyEmpty(client *goredis.Client, packageName string, exitCode int) int {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	remaining, err := client.DBSize(ctx).Result()
	if err != nil {
		fmt.Fprintf(os.Stderr, "verify %s Redis cleanup: %v\n", packageName, err)
		return 1
	}
	if remaining != 0 {
		// Leaked keys make later results depend on test order. Failing the package
		// here turns that hidden source of flakiness into an immediate signal.
		fmt.Fprintf(os.Stderr, "verify %s Redis cleanup: %d key(s) remain\n", packageName, remaining)
		return 1
	}
	return exitCode
}
