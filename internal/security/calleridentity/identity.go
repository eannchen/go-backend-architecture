package calleridentity

// AuthenticationType names the mechanism that verified the caller.
type AuthenticationType string

const (
	// AuthenticationTypeMTLS means a trusted client certificate authenticated the caller.
	AuthenticationTypeMTLS AuthenticationType = "mtls"
)

// Identity describes an authenticated machine caller independently of its transport.
type Identity struct {
	// Subject is the canonical, stable name derived from verified credentials.
	Subject string
	// AuthenticationType records how the subject was verified, not what it may do.
	AuthenticationType AuthenticationType
}
