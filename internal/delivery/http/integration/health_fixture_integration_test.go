//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	httpdelivery "github.com/eannchen/go-backend-architecture/internal/delivery/http"
	healthhttp "github.com/eannchen/go-backend-architecture/internal/delivery/http/handler/health"
	openapi "github.com/eannchen/go-backend-architecture/internal/delivery/http/openapi/gen"
	httpresponse "github.com/eannchen/go-backend-architecture/internal/delivery/http/response"
	rediscachestore "github.com/eannchen/go-backend-architecture/internal/infra/cache/redis/store"
	postgresstore "github.com/eannchen/go-backend-architecture/internal/infra/db/postgres/store"
	rediskvstore "github.com/eannchen/go-backend-architecture/internal/infra/kvstore/redis/store"
	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/observability"
	usecasehealth "github.com/eannchen/go-backend-architecture/internal/usecase/health"
)

type healthFixture struct {
	*serverFixture
}

func newHealthFixture(t *testing.T) *healthFixture {
	t.Helper()

	// Health checks are read-only, so this fixture needs no per-test row or key cleanup.
	tracer := observability.NoopTracer{}
	meter := observability.NoopMeter{}
	dbHealth := postgresstore.NewDBHealthStore(postgresPool, tracer)
	cacheHealth := rediscachestore.NewHealthStore(redisClient)
	kvHealth := rediskvstore.NewHealthStore(redisClient)
	usecase := usecasehealth.New(tracer, meter, dbHealth, cacheHealth, kvHealth)
	handler := healthhttp.NewHandler(
		logger.NoopLogger{},
		tracer,
		httpresponse.NewResponder(nil),
		usecase,
		healthhttp.StreamConfig{
			CheckInterval:     time.Minute,
			HeartbeatInterval: time.Minute,
			MaxDuration:       2 * time.Minute,
		},
	)

	return &healthFixture{
		serverFixture: newServerFixture(t, []httpdelivery.ValidationRegistrar{healthhttp.RegisterValidation}, handler),
	}
}

func (f *healthFixture) getHealth(t *testing.T, query string) openapi.HealthResponse {
	t.Helper()

	path := "/health"
	if query != "" {
		path += "?check=" + query
	}
	response := f.sendJSON(t, http.MethodGet, path, nil, nil, http.StatusOK)

	var decoded openapi.HealthResponse
	if err := json.Unmarshal(response.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	return decoded
}
