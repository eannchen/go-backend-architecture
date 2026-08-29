package otel

import (
	"context"
	"testing"

	"github.com/eannchen/go-backend-architecture/internal/infra/config"
	"github.com/eannchen/go-backend-architecture/internal/observability"
)

func TestSetup_DisabledReturnsNoopRuntime(t *testing.T) {
	runtime, err := Setup(context.Background(), config.OTelConfig{Enabled: false}, "accounts-api", "test")
	if err != nil {
		t.Fatalf("setup disabled observability: %v", err)
	}
	if _, ok := runtime.(observability.NoopRuntime); !ok {
		t.Fatalf("runtime type = %T, want observability.NoopRuntime", runtime)
	}
}
