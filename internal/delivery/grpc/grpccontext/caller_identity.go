package grpccontext

// AuthenticationType names the mechanism that verified the caller.
type AuthenticationType string

const (
	// AuthenticationTypeMTLS means a trusted client certificate authenticated the caller.
	AuthenticationTypeMTLS AuthenticationType = "mtls"
)

// CallerIdentity describes an authenticated machine caller attached to a gRPC call.
type CallerIdentity struct {
	// Subject is the canonical, stable name derived from verified credentials.
	Subject string
	// AuthenticationType records how the subject was verified, not what it may do.
	AuthenticationType AuthenticationType
}
