package grpccontext

import "context"

type callerIdentityKey struct{}

// WithCallerIdentity returns a context carrying an authenticated caller identity.
func WithCallerIdentity(ctx context.Context, identity CallerIdentity) context.Context {
	return context.WithValue(ctx, callerIdentityKey{}, identity)
}

// CallerIdentityFromContext returns the authenticated caller identity, when one was established.
func CallerIdentityFromContext(ctx context.Context) (CallerIdentity, bool) {
	identity, ok := ctx.Value(callerIdentityKey{}).(CallerIdentity)
	return identity, ok
}
