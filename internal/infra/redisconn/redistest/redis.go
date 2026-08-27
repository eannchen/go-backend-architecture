//go:build integration

// Package redistest owns disposable Redis infrastructure for integration tests.
package redistest

import (
	"context"
	"errors"
	"fmt"
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

type packageRedis struct {
	container testcontainers.Container
	client    *goredis.Client
}

// RunPackage owns the complete Redis lifecycle around one package's tests.
func RunPackage(m *testing.M, packageName string, setClient func(*goredis.Client)) int {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	testRedis, err := startPackage(ctx)
	if err != nil {
		// The integration build tag is an explicit request to exercise Redis. If
		// Docker is unavailable, failing prevents CI from reporting a false success.
		fmt.Fprintf(os.Stderr, "start %s Redis integration dependency: %v\n", packageName, err)
		return 1
	}
	setClient(testRedis.Client())

	exitCode := m.Run()
	exitCode = verifyEmpty(testRedis.Client(), packageName, exitCode)
	if err := testRedis.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "close %s Redis integration dependency: %v\n", packageName, err)
		if exitCode == 0 {
			exitCode = 1
		}
	}
	return exitCode
}

func startPackage(ctx context.Context) (*packageRedis, error) {
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

	return &packageRedis{container: container, client: client}, nil
}

func (r *packageRedis) Client() *goredis.Client {
	return r.client
}

func (r *packageRedis) Close() error {
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
