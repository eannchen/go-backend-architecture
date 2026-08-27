package recovery

import (
	"context"
	"errors"
	"strings"
	"testing"

	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/eannchen/go-backend-architecture/internal/logger/loggertest"
)

func TestUnaryRecoversPanic(t *testing.T) {
	log := &loggertest.Logger{}
	_, err := New(log, nil).Unary()(context.Background(), nil, &googlegrpc.UnaryServerInfo{FullMethod: "/test.Service/Panic"}, func(context.Context, any) (any, error) {
		panic("secret panic")
	})

	if status.Code(err) != codes.Internal || status.Convert(err).Message() != "internal server error" {
		t.Fatalf("error = %v, want safe Internal status", err)
	}
	if log.ErrorCalls != 1 || !strings.Contains(log.Errors[0].Err.Error(), "secret panic") {
		t.Fatalf("error logs = %#v, want one", log.Errors)
	}
	if got := log.Errors[0].Fields[0]["panic.stack"]; got == "" {
		t.Fatal("expected panic stack in internal log")
	}
}

func TestStreamRecoversPanic(t *testing.T) {
	log := &loggertest.Logger{}
	err := New(log, nil).Stream()(nil, recoveryServerStream{}, &googlegrpc.StreamServerInfo{FullMethod: "/test.Service/Watch"}, func(any, googlegrpc.ServerStream) error {
		panic(errors.New("stream panic"))
	})

	if status.Code(err) != codes.Internal || log.ErrorCalls != 1 {
		t.Fatalf("error = %v, error logs = %d", err, log.ErrorCalls)
	}
}

type recoveryServerStream struct{}

func (recoveryServerStream) SetHeader(metadata.MD) error  { return nil }
func (recoveryServerStream) SendHeader(metadata.MD) error { return nil }
func (recoveryServerStream) SetTrailer(metadata.MD)       {}
func (recoveryServerStream) Context() context.Context     { return context.Background() }
func (recoveryServerStream) SendMsg(any) error            { return nil }
func (recoveryServerStream) RecvMsg(any) error            { return nil }
