package health

import (
	"context"
	"fmt"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
	"github.com/eannchen/go-backend-architecture/pkg/observability"
)

type Result struct {
	Database Database
	Cache    Dependency
	KVStore  Dependency
}

type Database struct {
	Status        string
	Name          string
	InRecovery    bool
	UptimeSeconds int64
}

type Dependency struct {
	Status string
}

// Usecase implements the health workflow.
type Usecase struct {
	tracer       observability.Tracer
	checkTotal   observability.Counter
	checkLatency observability.Histogram
	dbHealth     DatabaseHealth
	cacheHealth  Pinger
	kvHealth     Pinger
}

func New(
	tracer observability.Tracer,
	meter observability.Meter,
	dbHealth DatabaseHealth,
	cacheHealth Pinger,
	kvHealth Pinger,
) *Usecase {
	return &Usecase{
		tracer: tracer,
		checkTotal: meter.Counter("app.health.checks",
			observability.MetricOption{
				Description: "Total number of health checks by mode and result.",
				Unit:        "{check}",
			},
		),
		checkLatency: meter.Histogram("app.health.check.duration",
			observability.MetricOption{
				Description: "Health check latency in seconds by mode and result.",
				Unit:        "s",
			},
		),
		dbHealth:    dbHealth,
		cacheHealth: cacheHealth,
		kvHealth:    kvHealth,
	}
}

func (u *Usecase) Check(ctx context.Context, mode CheckMode) (result Result, err error) {
	ctx, span := u.tracer.Start(ctx, "usecase", "health_usecase.check")
	startedAt := time.Now()
	defer func() {
		outcome := "success"
		if err != nil {
			outcome = "error"
		}
		fields := observability.FromPairs(
			"health.mode", string(mode),
			"health.outcome", outcome,
		)
		u.checkTotal.Add(ctx, 1, fields)
		u.checkLatency.Record(ctx, time.Since(startedAt).Seconds(), fields)
		span.Finish(observability.SpanOutcome{Err: err, Failed: err != nil})
	}()

	if mode == "" {
		mode = CheckModeReady
	}
	if !mode.IsValid() {
		return Result{}, apperr.New(apperr.CodeInvalidArgument, fmt.Sprintf("invalid health check mode: %q", mode))
	}

	result = Result{
		Database: Database{
			Status: "skipped",
		},
		Cache: Dependency{
			Status: "skipped",
		},
		KVStore: Dependency{
			Status: "skipped",
		},
	}

	if mode == CheckModeLive {
		return result, nil
	}

	if err := u.dbHealth.Ping(ctx); err != nil {
		result.Database.Status = "down"
		return result, apperr.Wrap(err, apperr.CodeUnavailable, "database readiness failed")
	}

	serverStatus, err := u.dbHealth.GetServerStatus(ctx)
	if err != nil {
		result.Database.Status = "down"
		return result, apperr.Wrap(err, apperr.CodeUnavailable, "database server status query failed")
	}

	result.Database.Status = "up"
	result.Database.Name = serverStatus.DatabaseName
	result.Database.InRecovery = serverStatus.InRecovery
	result.Database.UptimeSeconds = serverStatus.UptimeSeconds

	if err := u.cacheHealth.Ping(ctx); err != nil {
		result.Cache.Status = "down"
		return result, apperr.Wrap(err, apperr.CodeUnavailable, "cache readiness failed")
	}
	result.Cache.Status = "up"

	if err := u.kvHealth.Ping(ctx); err != nil {
		result.KVStore.Status = "down"
		return result, apperr.Wrap(err, apperr.CodeUnavailable, "kv readiness failed")
	}
	result.KVStore.Status = "up"

	return result, nil
}
