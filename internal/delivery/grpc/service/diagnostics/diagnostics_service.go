package diagnostics

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
	diagnosticsv1 "github.com/eannchen/go-backend-architecture/internal/delivery/grpc/gen/diagnostics/v1"
	grpcresponse "github.com/eannchen/go-backend-architecture/internal/delivery/grpc/response"
	usecasehealth "github.com/eannchen/go-backend-architecture/internal/usecase/health"
)

// Service maps the detailed diagnostics transport to the health usecase.
type Service struct {
	diagnosticsv1.UnimplementedDiagnosticsServiceServer
	health    usecasehealth.Usecase
	responder grpcresponse.Responder
}

func NewService(health usecasehealth.Usecase, responder grpcresponse.Responder) *Service {
	if responder == nil {
		responder = grpcresponse.NewResponder()
	}
	return &Service{health: health, responder: responder}
}

func (s *Service) GetHealth(ctx context.Context, req *diagnosticsv1.GetHealthRequest) (*diagnosticsv1.GetHealthResponse, error) {
	mode, err := toCheckMode(req.GetMode())
	if err != nil {
		return nil, s.responder.Error(err, codes.InvalidArgument, err.Error())
	}

	result, checkErr := s.health.Check(ctx, mode)
	if checkErr != nil {
		if errors.Is(checkErr, context.Canceled) || errors.Is(checkErr, context.DeadlineExceeded) {
			return nil, s.responder.AppError(checkErr)
		}
		appErr, ok := apperr.As(checkErr)
		if !ok || appErr.Code != apperr.CodeUnavailable {
			return nil, s.responder.AppError(checkErr)
		}
	}

	return toResponse(result, checkErr == nil), nil
}

func toCheckMode(mode diagnosticsv1.HealthCheckMode) (usecasehealth.CheckMode, error) {
	switch mode {
	case diagnosticsv1.HealthCheckMode_HEALTH_CHECK_MODE_UNSPECIFIED,
		diagnosticsv1.HealthCheckMode_HEALTH_CHECK_MODE_READY:
		return usecasehealth.CheckModeReady, nil
	case diagnosticsv1.HealthCheckMode_HEALTH_CHECK_MODE_LIVE:
		return usecasehealth.CheckModeLive, nil
	default:
		return "", fmt.Errorf("unsupported health check mode: %d", mode)
	}
}

func toResponse(result usecasehealth.Result, healthy bool) *diagnosticsv1.GetHealthResponse {
	database := &diagnosticsv1.DatabaseHealth{
		Status: toHealthStatus(result.Database.Status),
	}
	if result.Database.Status == "up" {
		database.Name = pointer(result.Database.Name)
		database.InRecovery = pointer(result.Database.InRecovery)
		database.UptimeSeconds = pointer(result.Database.UptimeSeconds)
	}

	return &diagnosticsv1.GetHealthResponse{
		Healthy:     healthy,
		Database:    database,
		Cache:       &diagnosticsv1.DependencyHealth{Status: toHealthStatus(result.Cache.Status)},
		KvStore:     &diagnosticsv1.DependencyHealth{Status: toHealthStatus(result.KVStore.Status)},
		VectorStore: &diagnosticsv1.DependencyHealth{Status: toHealthStatus(result.Vector.Status)},
	}
}

func toHealthStatus(value string) diagnosticsv1.HealthStatus {
	switch value {
	case "up":
		return diagnosticsv1.HealthStatus_HEALTH_STATUS_UP
	case "down":
		return diagnosticsv1.HealthStatus_HEALTH_STATUS_DOWN
	case "skipped":
		return diagnosticsv1.HealthStatus_HEALTH_STATUS_SKIPPED
	default:
		return diagnosticsv1.HealthStatus_HEALTH_STATUS_UNSPECIFIED
	}
}

func pointer[T any](value T) *T {
	return &value
}

var _ diagnosticsv1.DiagnosticsServiceServer = (*Service)(nil)
