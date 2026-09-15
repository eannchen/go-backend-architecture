package grpcapi

import (
	"fmt"

	googlegrpc "google.golang.org/grpc"
	grpcstandardhealth "google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	grpcdelivery "github.com/eannchen/go-backend-architecture/internal/delivery/grpc"
	diagnosticsv1 "github.com/eannchen/go-backend-architecture/internal/delivery/grpc/gen/diagnostics/v1"
	calleridentityinterceptor "github.com/eannchen/go-backend-architecture/internal/delivery/grpc/interceptor/calleridentity"
	observabilityinterceptor "github.com/eannchen/go-backend-architecture/internal/delivery/grpc/interceptor/observability"
	recoveryinterceptor "github.com/eannchen/go-backend-architecture/internal/delivery/grpc/interceptor/recovery"
	requestcontextinterceptor "github.com/eannchen/go-backend-architecture/internal/delivery/grpc/interceptor/requestcontext"
	grpcresponse "github.com/eannchen/go-backend-architecture/internal/delivery/grpc/response"
	diagnosticsservice "github.com/eannchen/go-backend-architecture/internal/delivery/grpc/service/diagnostics"
	healthservice "github.com/eannchen/go-backend-architecture/internal/delivery/grpc/service/health"
	infracalleridentity "github.com/eannchen/go-backend-architecture/internal/infra/security/calleridentity"
	usecasehealth "github.com/eannchen/go-backend-architecture/internal/usecase/health"
)

type serverComponents struct {
	server   *grpcdelivery.Server
	reporter *healthservice.Reporter
}

func (d wiring) buildServer(healthUsecase usecasehealth.Usecase) (serverComponents, error) {
	transportCredentials, err := d.buildTransportCredentials()
	if err != nil {
		return serverComponents{}, err
	}
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
	requestContext, err := requestcontextinterceptor.New(requestcontextinterceptor.Config{
		Timeout: d.cfg.GRPC.RequestTimeout,
		RequestID: requestcontextinterceptor.RequestIDConfig{
			IncomingMetadataKey: d.cfg.GRPC.RequestID.IncomingKey,
			ResponseMetadataKey: d.cfg.GRPC.RequestID.ResponseKey,
			RejectInvalid:       d.cfg.GRPC.RequestID.RejectInvalid,
		},
	}, responder)
	if err != nil {
		return serverComponents{}, fmt.Errorf("create gRPC request-context interceptor: %w", err)
	}
	requestObservability := observabilityinterceptor.New(d.tracer, d.log, d.meter)
	recovery := recoveryinterceptor.New(d.log, responder)
	unaryInterceptors := []googlegrpc.UnaryServerInterceptor{requestContext.Unary()}
	streamInterceptors := []googlegrpc.StreamServerInterceptor{requestContext.Stream()}
	if d.cfg.GRPC.TLS.Enabled && d.cfg.GRPC.TLS.ClientCAFile != "" {
		callerIdentity, err := calleridentityinterceptor.New(infracalleridentity.URICertificateExtractor{}, responder)
		if err != nil {
			return serverComponents{}, fmt.Errorf("create gRPC caller-identity interceptor: %w", err)
		}
		unaryInterceptors = append(unaryInterceptors, callerIdentity.Unary())
		streamInterceptors = append(streamInterceptors, callerIdentity.Stream())
	}
	unaryInterceptors = append(unaryInterceptors, requestObservability.Unary(), recovery.Unary())
	streamInterceptors = append(streamInterceptors, requestObservability.Stream(), recovery.Stream())

	// Interceptors are listed outermost to innermost. Context enrichers run before
	// observability because gRPC passes derived contexts inward only. Recovery
	// stays inside observability so recovered panics become failed RPC outcomes.
	server, err := grpcdelivery.NewServer(
		grpcdelivery.ServerConfig{
			Address:              d.cfg.GRPC.Address,
			ReflectionEnabled:    d.cfg.GRPC.ReflectionEnabled,
			MaxRecvMessageBytes:  d.cfg.GRPC.MaxRecvMessageBytes,
			MaxSendMessageBytes:  d.cfg.GRPC.MaxSendMessageBytes,
			TransportCredentials: transportCredentials,
		},
		d.log,
		unaryInterceptors,
		streamInterceptors,
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
