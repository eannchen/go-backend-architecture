package recovery

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/eannchen/go-backend-architecture/internal/delivery/kafka/middleware"
	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/logger/loggertest"

	"github.com/twmb/franz-go/pkg/kgo"
)

// TestRecovery contains handler panics and reports useful coordinates and stack
// locations without exposing the panic value, message payload, or message key.
func TestRecovery(t *testing.T) {
	log := &loggertest.Logger{ErrorFunc: func(context.Context, string, error, ...logger.Fields) {}}
	handler := New(log).Handler()(func(context.Context, *kgo.Record) error { panic("private-panic") })
	err := handler(t.Context(), &kgo.Record{Topic: "jobs", Partition: 1, Offset: 3, Value: []byte("private-body"), Key: []byte("private-key")})
	if !errors.Is(err, middleware.ErrHandlerPanic) || len(log.ErrorCalls) != 1 {
		t.Fatal("panic escaped recovery")
	}
	fields := logger.OptionalFields(log.ErrorCalls[0].Fields...)
	if fields["messaging.kafka.offset"] != int64(3) || !strings.Contains(fmt.Sprint(fields["panic.stack"]), "TestRecovery") {
		t.Fatal("missing recovery diagnostics")
	}
	if strings.Contains(fmt.Sprint(fields), "private-") {
		t.Fatal("private data logged")
	}
	failure := errors.New("normal failure")
	passthrough := New(log).Handler()(func(context.Context, *kgo.Record) error { return failure })
	if !errors.Is(passthrough(t.Context(), &kgo.Record{}), failure) || len(log.ErrorCalls) != 1 {
		t.Fatal("ordinary failure treated as panic")
	}
}
