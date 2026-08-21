package health

import (
	"context"

	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	usecasehealth "github.com/eannchen/go-backend-architecture/internal/usecase/health"
)

const overallService = ""

// ServingStatusSetter is the part of the standard gRPC health server used by Reporter.
type ServingStatusSetter interface {
	SetServingStatus(string, healthpb.HealthCheckResponse_ServingStatus)
}

// Reporter translates readiness results into cached standard gRPC health states.
type Reporter struct {
	health   usecasehealth.Usecase
	setter   ServingStatusSetter
	services []string
}

func NewReporter(health usecasehealth.Usecase, setter ServingStatusSetter, serviceName string) *Reporter {
	services := []string{overallService}
	setter.SetServingStatus(overallService, healthpb.HealthCheckResponse_NOT_SERVING)

	if serviceName != overallService {
		services = append(services, serviceName)
		setter.SetServingStatus(serviceName, healthpb.HealthCheckResponse_NOT_SERVING)
	}

	return &Reporter{health: health, setter: setter, services: services}
}

// Refresh runs one readiness check and publishes its state to the standard health service.
func (r *Reporter) Refresh(ctx context.Context) error {
	servingStatus := healthpb.HealthCheckResponse_SERVING

	_, err := r.health.Check(ctx, usecasehealth.CheckModeReady)
	if err != nil {
		servingStatus = healthpb.HealthCheckResponse_NOT_SERVING
	}

	for _, service := range r.services {
		r.setter.SetServingStatus(service, servingStatus)
	}

	return err
}
