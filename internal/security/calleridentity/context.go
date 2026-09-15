package calleridentity

import "context"

type contextKey struct{}

// WithIdentity returns a context carrying an authenticated caller identity.
func WithIdentity(ctx context.Context, identity Identity) context.Context {
	return context.WithValue(ctx, contextKey{}, identity)
}

// FromContext returns the authenticated caller identity, when one was established.
func FromContext(ctx context.Context) (Identity, bool) {
	identity, ok := ctx.Value(contextKey{}).(Identity)
	return identity, ok
}
