package requestcontext

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/eannchen/go-backend-architecture/internal/observability"
)

const testRequestIDMetadataKey = "correlation-id"

// TestUnaryAppliesDeadlineWithoutRequestIDMetadataByDefault checks default deadlines apply without adding request ID metadata.
func TestUnaryAppliesDeadlineWithoutRequestIDMetadataByDefault(t *testing.T) {
	err := newTestInterceptor(t, Config{Timeout: time.Second}).Unary()(context.Background(), "/test.Service/Check", nil, nil, nil, func(ctx context.Context, _ string, _, _ any, _ *googlegrpc.ClientConn, _ ...googlegrpc.CallOption) error {
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("expected default deadline")
		}
		md, _ := metadata.FromOutgoingContext(ctx)
		if got := md.Get(testRequestIDMetadataKey); len(got) != 0 {
			t.Fatalf("request ID metadata = %v, want none", got)
		}
		if requestID := observability.RequestIDFromContext(ctx); requestID != "" {
			t.Fatalf("request ID = %q, want none", requestID)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Unary() error = %v", err)
	}
}

// TestUnaryPropagatesConfiguredRequestID checks configured request IDs propagate to outbound unary calls.
func TestUnaryPropagatesConfiguredRequestID(t *testing.T) {
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer token"))
	ctx = observability.WithRequestID(ctx, "request-123")
	err := newTestInterceptor(t, Config{Timeout: time.Second, RequestIDMetadataKey: testRequestIDMetadataKey}).Unary()(ctx, "/test.Service/Check", nil, nil, nil, func(ctx context.Context, _ string, _, _ any, _ *googlegrpc.ClientConn, _ ...googlegrpc.CallOption) error {
		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok || len(md.Get(testRequestIDMetadataKey)) != 1 || md.Get(testRequestIDMetadataKey)[0] != "request-123" {
			t.Fatalf("outgoing metadata = %#v", md)
		}
		if got := md.Get("authorization"); len(got) != 1 || got[0] != "Bearer token" {
			t.Fatalf("preserved authorization metadata = %v", got)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Unary() error = %v", err)
	}
}

// TestUnaryDoesNotGenerateRequestIDForBackgroundCall checks background calls do not invent request IDs.
func TestUnaryDoesNotGenerateRequestIDForBackgroundCall(t *testing.T) {
	err := newTestInterceptor(t, Config{Timeout: time.Second, RequestIDMetadataKey: testRequestIDMetadataKey}).Unary()(context.Background(), "/test.Service/Check", nil, nil, nil, func(ctx context.Context, _ string, _, _ any, _ *googlegrpc.ClientConn, _ ...googlegrpc.CallOption) error {
		requestID := observability.RequestIDFromContext(ctx)
		md, _ := metadata.FromOutgoingContext(ctx)
		if requestID != "" || len(md.Get(testRequestIDMetadataKey)) != 0 {
			t.Fatalf("request ID = %q, metadata = %#v; want neither", requestID, md)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Unary() error = %v", err)
	}
}

// TestUnaryPreservesEarlierCallerDeadline checks a shorter caller deadline is preserved.
func TestUnaryPreservesEarlierCallerDeadline(t *testing.T) {
	callerDeadline := time.Now().Add(50 * time.Millisecond)
	ctx, cancel := context.WithDeadline(context.Background(), callerDeadline)
	defer cancel()

	err := newTestInterceptor(t, Config{Timeout: time.Second}).Unary()(ctx, "/test.Service/Check", nil, nil, nil, func(ctx context.Context, _ string, _, _ any, _ *googlegrpc.ClientConn, _ ...googlegrpc.CallOption) error {
		got, ok := ctx.Deadline()
		if !ok || !got.Equal(callerDeadline) {
			t.Fatalf("deadline = %v, want caller deadline %v", got, callerDeadline)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Unary() error = %v", err)
	}
}

// TestStreamKeepsContextUntilReceiveCompletes checks stream context stays live until receiving finishes.
func TestStreamKeepsContextUntilReceiveCompletes(t *testing.T) {
	base := &testClientStream{ctx: context.Background(), recvErr: io.EOF}
	stream, err := newTestInterceptor(t, Config{Timeout: time.Second}).Stream()(context.Background(), &googlegrpc.StreamDesc{ServerStreams: true}, nil, "/test.Service/Watch", func(ctx context.Context, _ *googlegrpc.StreamDesc, _ *googlegrpc.ClientConn, _ string, _ ...googlegrpc.CallOption) (googlegrpc.ClientStream, error) {
		base.ctx = ctx
		return base, nil
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	if base.ctx.Err() != nil {
		t.Fatalf("stream context ended before receive: %v", base.ctx.Err())
	}
	if err := stream.RecvMsg(nil); err != io.EOF {
		t.Fatalf("RecvMsg() error = %v, want EOF", err)
	}
	if base.ctx.Err() == nil {
		t.Fatal("stream context was not canceled after completion")
	}
}

// TestStreamCancelsContextAfterSendFailure checks send failures cancel the stream context promptly.
func TestStreamCancelsContextAfterSendFailure(t *testing.T) {
	wantErr := errors.New("send failed")
	base := &testClientStream{ctx: context.Background(), sendErr: wantErr}
	stream, err := newTestInterceptor(t, Config{Timeout: time.Second}).Stream()(context.Background(), &googlegrpc.StreamDesc{ClientStreams: true}, nil, "/test.Service/Upload", func(ctx context.Context, _ *googlegrpc.StreamDesc, _ *googlegrpc.ClientConn, _ string, _ ...googlegrpc.CallOption) (googlegrpc.ClientStream, error) {
		base.ctx = ctx
		return base, nil
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	if err := stream.SendMsg(nil); !errors.Is(err, wantErr) {
		t.Fatalf("SendMsg() error = %v, want %v", err, wantErr)
	}
	if base.ctx.Err() == nil {
		t.Fatal("stream context was not canceled after send failure")
	}
}

// TestNewRejectsInvalidMetadataKey checks invalid metadata keys fail when creating the interceptor.
func TestNewRejectsInvalidMetadataKey(t *testing.T) {
	if _, err := New(Config{RequestIDMetadataKey: "request id"}); err == nil {
		t.Fatal("New() error = nil, want invalid metadata key error")
	}
}

func newTestInterceptor(t *testing.T, config Config) *Interceptor {
	t.Helper()
	interceptor, err := New(config)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return interceptor
}

type testClientStream struct {
	googlegrpc.ClientStream
	ctx     context.Context
	sendErr error
	recvErr error
}

func (s *testClientStream) Context() context.Context { return s.ctx }
func (s *testClientStream) SendMsg(any) error        { return s.sendErr }
func (s *testClientStream) RecvMsg(any) error        { return s.recvErr }
