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

func TestUnaryPropagatesRequestIDAndDeadline(t *testing.T) {
	interceptor := New(time.Minute, nil).Unary()
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(RequestIDMetadataKey, "request-01"))
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
	if got := transport.header.Get(RequestIDMetadataKey); len(got) != 1 || got[0] != "request-01" {
		t.Fatalf("response request ID = %v, want request-01", got)
	}
}

func TestUnaryPreservesEarlierClientDeadline(t *testing.T) {
	clientDeadline := 20 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), clientDeadline)
	defer cancel()
	ctx = googlegrpc.NewContextWithServerTransportStream(ctx, &recordingServerTransportStream{})

	_, err := New(time.Minute, nil).Unary()(ctx, nil, &googlegrpc.UnaryServerInfo{}, func(ctx context.Context, _ any) (any, error) {
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

func TestUnaryGeneratesRequestID(t *testing.T) {
	transport := &recordingServerTransportStream{}
	ctx := googlegrpc.NewContextWithServerTransportStream(context.Background(), transport)
	_, err := New(0, nil).Unary()(ctx, nil, &googlegrpc.UnaryServerInfo{}, func(ctx context.Context, _ any) (any, error) {
		if !observability.IsValidRequestID(observability.RequestIDFromContext(ctx)) {
			t.Fatal("expected generated request ID in context")
		}
		return nil, nil
	})
	if err != nil {
		t.Fatalf("interceptor error = %v", err)
	}
	if got := transport.header.Get(RequestIDMetadataKey); len(got) != 1 || !observability.IsValidRequestID(got[0]) {
		t.Fatalf("generated response request ID = %v", got)
	}
}

func TestUnaryRejectsInvalidOrRepeatedRequestID(t *testing.T) {
	tests := []struct {
		name   string
		values []string
	}{
		{name: "invalid characters", values: []string{"not valid"}},
		{name: "repeated", values: []string{"first", "second"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{RequestIDMetadataKey: tt.values})
			called := false
			_, err := New(0, nil).Unary()(ctx, nil, &googlegrpc.UnaryServerInfo{}, func(context.Context, any) (any, error) {
				called = true
				return nil, nil
			})
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("status = %v, want InvalidArgument", status.Code(err))
			}
			if called {
				t.Fatal("handler called for invalid request ID")
			}
		})
	}
}

func TestUnaryReturnsHandlerErrorUnchanged(t *testing.T) {
	handlerErr := errors.Join(errors.New("operation timed out"), context.DeadlineExceeded)
	ctx := googlegrpc.NewContextWithServerTransportStream(context.Background(), &recordingServerTransportStream{})
	_, err := New(0, nil).Unary()(ctx, nil, &googlegrpc.UnaryServerInfo{}, func(context.Context, any) (any, error) {
		return nil, handlerErr
	})
	if err != handlerErr {
		t.Fatalf("error = %v, want unchanged handler error", err)
	}
}

func TestUnaryTimeoutCancelsHandlerAndReturnsItsError(t *testing.T) {
	ctx := googlegrpc.NewContextWithServerTransportStream(context.Background(), &recordingServerTransportStream{})
	_, err := New(time.Millisecond, nil).Unary()(ctx, nil, &googlegrpc.UnaryServerInfo{}, func(ctx context.Context, _ any) (any, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want raw DeadlineExceeded from handler", err)
	}
}

func TestStreamPropagatesRequestIDWithoutAddingDeadline(t *testing.T) {
	base := &testServerStream{ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs(RequestIDMetadataKey, "stream-01"))}
	err := New(time.Millisecond, nil).Stream()(nil, base, &googlegrpc.StreamServerInfo{}, func(_ any, stream googlegrpc.ServerStream) error {
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
	if got := base.header.Get(RequestIDMetadataKey); len(got) != 1 || got[0] != "stream-01" {
		t.Fatalf("response request ID = %v, want stream-01", got)
	}
}

func TestStreamReturnsHandlerErrorUnchanged(t *testing.T) {
	handlerErr := errors.New("service mapped error")
	base := &testServerStream{ctx: context.Background()}

	err := New(0, nil).Stream()(nil, base, &googlegrpc.StreamServerInfo{}, func(any, googlegrpc.ServerStream) error {
		return handlerErr
	})
	if err != handlerErr {
		t.Fatalf("error = %v, want unchanged handler error", err)
	}
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
