package grpcapi

import (
	googlegrpc "google.golang.org/grpc"
	grpcstandardhealth "google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	grpcdelivery "github.com/eannchen/go-backend-architecture/internal/delivery/grpc"
	diagnosticsv1 "github.com/eannchen/go-backend-architecture/internal/delivery/grpc/gen/diagnostics/v1"
	grpcresponse "github.com/eannchen/go-backend-architecture/internal/delivery/grpc/response"
	diagnosticsservice "github.com/eannchen/go-backend-architecture/internal/delivery/grpc/service/diagnostics"
	healthservice "github.com/eannchen/go-backend-architecture/internal/delivery/grpc/service/health"
	usecasehealth "github.com/eannchen/go-backend-architecture/internal/usecase/health"
)

type serverComponents struct {
	server   *grpcdelivery.Server
	reporter *healthservice.Reporter
}

func (d wiring) buildServer(healthUsecase usecasehealth.Usecase) (serverComponents, error) {
	responder := grpcresponse.NewResponder()
	diagnostics := diagnosticsservice.NewService(healthUsecase, responder)
	standardHealth := grpcstandardhealth.NewServer()
	reporter, err := healthservice.NewReporter(
		healthservice.ReporterConfig{
			RefreshInterval: d.cfg.GRPC.HealthRefreshInterval,
		},
		d.log,
		healthUsecase,
		standardHealth,
		diagnosticsv1.DiagnosticsService_ServiceDesc.ServiceName,
	)
	if err != nil {
		return serverComponents{}, err
	}

	server, err := grpcdelivery.NewServer(
		grpcdelivery.ServerConfig{
			Address:           d.cfg.GRPC.Address,
			ReflectionEnabled: d.cfg.GRPC.ReflectionEnabled,
		},
		d.log,
		nil,
		nil,
		grpcdelivery.ServiceRegistrarFunc(func(registrar googlegrpc.ServiceRegistrar) {
			diagnosticsv1.RegisterDiagnosticsServiceServer(registrar, diagnostics)
		}),
		grpcdelivery.ServiceRegistrarFunc(func(registrar googlegrpc.ServiceRegistrar) {
			healthpb.RegisterHealthServer(registrar, standardHealth)
		}),
	)
	if err != nil {
		return serverComponents{}, err
	}

	return serverComponents{
		server:   server,
		reporter: reporter,
	}, nil
}
