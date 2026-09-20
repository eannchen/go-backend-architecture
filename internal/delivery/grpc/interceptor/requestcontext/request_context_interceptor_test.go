package requestcontext

import (
	"context"
	"errors"
	"testing"
	"time"

	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/eannchen/go-backend-architecture/internal/observability"
)

const (
	testRequestIDMetadataKey         = "correlation-id"
	testResponseRequestIDMetadataKey = "response-id"
)

// TestUnaryPropagatesRequestIDAndDeadline checks valid request metadata and server timeouts reach unary handlers.
func TestUnaryPropagatesRequestIDAndDeadline(t *testing.T) {
	interceptor := newTestInterceptor(t, conventionalConfig(time.Minute)).Unary()
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(testRequestIDMetadataKey, "request-01"))
	transport := &recordingServerTransportStream{}
	ctx = googlegrpc.NewContextWithServerTransportStream(ctx, transport)

	_, err := interceptor(ctx, nil, &googlegrpc.UnaryServerInfo{FullMethod: "/test.Service/Check"}, func(ctx context.Context, _ any) (any, error) {
		if got := observability.RequestIDFromContext(ctx); got != "request-01" {
			t.Fatalf("request ID = %q, want request-01", got)
		}
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("expected server deadline")
		}
		return nil, nil
	})
	if err != nil {
		t.Fatalf("interceptor error = %v", err)
	}
	if got := transport.header.Get(testResponseRequestIDMetadataKey); len(got) != 1 || got[0] != "request-01" {
		t.Fatalf("response request ID = %v, want request-01", got)
	}
}

// TestUnaryPreservesEarlierClientDeadline checks a shorter client deadline is never extended by server policy.
func TestUnaryPreservesEarlierClientDeadline(t *testing.T) {
	clientDeadline := 20 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), clientDeadline)
	defer cancel()
	ctx = googlegrpc.NewContextWithServerTransportStream(ctx, &recordingServerTransportStream{})

	_, err := newTestInterceptor(t, Config{Timeout: time.Minute}).Unary()(ctx, nil, &googlegrpc.UnaryServerInfo{}, func(ctx context.Context, _ any) (any, error) {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > clientDeadline {
			t.Fatalf("deadline = %v, want no later than client deadline", deadline)
		}
		return nil, nil
	})
	if err != nil {
		t.Fatalf("interceptor error = %v", err)
	}
}

// TestUnaryDoesNotGenerateRequestIDWhenMissing checks a missing request ID stays absent rather than inventing caller metadata.
func TestUnaryDoesNotGenerateRequestIDWhenMissing(t *testing.T) {
	transport := &recordingServerTransportStream{}
	ctx := googlegrpc.NewContextWithServerTransportStream(context.Background(), transport)
	_, err := newTestInterceptor(t, Config{}).Unary()(ctx, nil, &googlegrpc.UnaryServerInfo{}, func(ctx context.Context, _ any) (any, error) {
		if got := observability.RequestIDFromContext(ctx); got != "" {
			t.Fatalf("request ID = %q, want none", got)
		}
		return nil, nil
	})
	if err != nil {
		t.Fatalf("interceptor error = %v", err)
	}
	if len(transport.header) != 0 {
		t.Fatalf("response metadata = %v, want none", transport.header)
	}
}

// TestUnaryIgnoresInvalidOrRepeatedRequestIDByDefault checks default policy ignores unusable request IDs while still serving the call.
func TestUnaryIgnoresInvalidOrRepeatedRequestIDByDefault(t *testing.T) {
	tests := []struct {
		name   string
		values []string
	}{
		{name: "invalid characters", values: []string{"not valid"}},
		{name: "repeated", values: []string{"first", "second"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := &recordingServerTransportStream{}
			ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{testRequestIDMetadataKey: tt.values})
			ctx = googlegrpc.NewContextWithServerTransportStream(ctx, transport)
			_, err := newTestInterceptor(t, conventionalConfig(0)).Unary()(ctx, nil, &googlegrpc.UnaryServerInfo{}, func(ctx context.Context, _ any) (any, error) {
				if got := observability.RequestIDFromContext(ctx); got != "" {
					t.Fatalf("request ID = %q, want ignored", got)
				}
				return nil, nil
			})
			if err != nil {
				t.Fatalf("interceptor error = %v", err)
			}
			if len(transport.header) != 0 {
				t.Fatalf("response metadata = %v, want none", transport.header)
			}
		})
	}
}

// TestUnaryRejectsInvalidRequestIDWhenConfigured checks strict policy rejects invalid request IDs before invoking the handler.
func TestUnaryRejectsInvalidRequestIDWhenConfigured(t *testing.T) {
	config := conventionalConfig(0)
	config.RequestID.RejectInvalid = true
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(testRequestIDMetadataKey, "not valid"))
	called := false

	_, err := newTestInterceptor(t, config).Unary()(ctx, nil, &googlegrpc.UnaryServerInfo{}, func(context.Context, any) (any, error) {
		called = true
		return nil, nil
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("status = %v, want InvalidArgument", status.Code(err))
	}
	if called {
		t.Fatal("handler called for invalid request ID")
	}
}

// TestNewRejectsInvalidMetadataKey checks invalid metadata keys fail during construction rather than at request time.
func TestNewRejectsInvalidMetadataKey(t *testing.T) {
	_, err := New(Config{RequestID: RequestIDConfig{IncomingMetadataKey: "request id"}}, nil)
	if err == nil {
		t.Fatal("New() error = nil, want invalid metadata key error")
	}
}

// TestUnaryReturnsHandlerErrorUnchanged checks ordinary handler failures pass through without interception changes.
func TestUnaryReturnsHandlerErrorUnchanged(t *testing.T) {
	handlerErr := errors.Join(errors.New("operation timed out"), context.DeadlineExceeded)
	ctx := googlegrpc.NewContextWithServerTransportStream(context.Background(), &recordingServerTransportStream{})
	_, err := newTestInterceptor(t, Config{}).Unary()(ctx, nil, &googlegrpc.UnaryServerInfo{}, func(context.Context, any) (any, error) {
		return nil, handlerErr
	})
	if err != handlerErr {
		t.Fatalf("error = %v, want unchanged handler error", err)
	}
}

// TestUnaryTimeoutCancelsHandlerAndReturnsItsError checks timeout cancellation reaches the handler and its error is returned.
func TestUnaryTimeoutCancelsHandlerAndReturnsItsError(t *testing.T) {
	ctx := googlegrpc.NewContextWithServerTransportStream(context.Background(), &recordingServerTransportStream{})
	_, err := newTestInterceptor(t, Config{Timeout: time.Millisecond}).Unary()(ctx, nil, &googlegrpc.UnaryServerInfo{}, func(ctx context.Context, _ any) (any, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want raw DeadlineExceeded from handler", err)
	}
}

// TestStreamPropagatesRequestIDWithoutAddingDeadline checks stream handlers receive request ID without imposing a unary deadline.
func TestStreamPropagatesRequestIDWithoutAddingDeadline(t *testing.T) {
	base := &testServerStream{ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs(testRequestIDMetadataKey, "stream-01"))}
	err := newTestInterceptor(t, conventionalConfig(time.Millisecond)).Stream()(nil, base, &googlegrpc.StreamServerInfo{}, func(_ any, stream googlegrpc.ServerStream) error {
		if got := observability.RequestIDFromContext(stream.Context()); got != "stream-01" {
			t.Fatalf("request ID = %q, want stream-01", got)
		}
		if _, ok := stream.Context().Deadline(); ok {
			t.Fatal("stream should not receive the unary timeout")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("interceptor error = %v", err)
	}
	if got := base.header.Get(testResponseRequestIDMetadataKey); len(got) != 1 || got[0] != "stream-01" {
		t.Fatalf("response request ID = %v, want stream-01", got)
	}
}

// TestStreamReturnsHandlerErrorUnchanged checks stream handler failures pass through unchanged.
func TestStreamReturnsHandlerErrorUnchanged(t *testing.T) {
	handlerErr := errors.New("service mapped error")
	base := &testServerStream{ctx: context.Background()}

	err := newTestInterceptor(t, Config{}).Stream()(nil, base, &googlegrpc.StreamServerInfo{}, func(any, googlegrpc.ServerStream) error {
		return handlerErr
	})
	if err != handlerErr {
		t.Fatalf("error = %v, want unchanged handler error", err)
	}
}

func conventionalConfig(timeout time.Duration) Config {
	return Config{
		Timeout: timeout,
		RequestID: RequestIDConfig{
			IncomingMetadataKey: testRequestIDMetadataKey,
			ResponseMetadataKey: testResponseRequestIDMetadataKey,
		},
	}
}

func newTestInterceptor(t *testing.T, config Config) *Interceptor {
	t.Helper()
	interceptor, err := New(config, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return interceptor
}

type recordingServerTransportStream struct {
	header metadata.MD
}

func (*recordingServerTransportStream) Method() string { return "/test.Service/Check" }
func (s *recordingServerTransportStream) SetHeader(md metadata.MD) error {
	s.header = metadata.Join(s.header, md)
	return nil
}
func (*recordingServerTransportStream) SendHeader(metadata.MD) error { return nil }
func (*recordingServerTransportStream) SetTrailer(metadata.MD) error { return nil }

type testServerStream struct {
	ctx    context.Context
	header metadata.MD
}

func (s *testServerStream) SetHeader(md metadata.MD) error {
	s.header = metadata.Join(s.header, md)
	return nil
}
func (*testServerStream) SendHeader(metadata.MD) error { return nil }
func (*testServerStream) SetTrailer(metadata.MD)       {}
func (s *testServerStream) Context() context.Context   { return s.ctx }
func (*testServerStream) SendMsg(any) error            { return nil }
func (*testServerStream) RecvMsg(any) error            { return nil }
