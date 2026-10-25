package kafka

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/eannchen/go-backend-architecture/internal/usecase/notification"
	"github.com/eannchen/go-backend-architecture/internal/usecase/notification/notificationtest"
)

type readerStub struct {
	fetch  func(context.Context) (*kgo.Record, error)
	commit func(context.Context, ...*kgo.Record) error
	allow  func()
}

func (r readerStub) FetchRecord(ctx context.Context) (*kgo.Record, error) { return r.fetch(ctx) }
func (r readerStub) CommitRecords(ctx context.Context, messages ...*kgo.Record) error {
	return r.commit(ctx, messages...)
}

func (r readerStub) AllowRebalance() {
	if r.allow != nil {
		r.allow()
	}
}

// TestNotificationWorkerProcessing protects handle-before-commit ordering and
// ensures failures cannot advance group progress and always release rebalances.
func TestNotificationWorkerProcessing(t *testing.T) {
	failure := errors.New("dependency failed")
	valid := []byte(`{"request_id":"req-1","recipient":"user-1","content":"hello"}`)
	for _, tc := range []struct {
		name                 string
		payload              []byte
		handleErr, commitErr error
		want                 []string
		wantErr              error
	}{
		{name: "success", payload: valid, want: []string{"fetch", "handle", "commit", "release", "fetch"}},
		{name: "invalid JSON", payload: []byte(`{"content":`), want: []string{"fetch", "release"}},
		{name: "missing recipient", payload: []byte(`{"request_id":"r","content":"hello"}`), want: []string{"fetch", "release"}},
		{name: "handler failure", payload: valid, handleErr: failure, want: []string{"fetch", "handle", "release"}, wantErr: failure},
		{name: "commit failure", payload: valid, commitErr: failure, want: []string{"fetch", "handle", "commit", "release"}, wantErr: failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var steps []string
			message := &kgo.Record{Topic: "notifications", Partition: 2, Offset: 7, Value: tc.payload}
			fetches := 0
			reader := readerStub{
				allow: func() { steps = append(steps, "release") },
				fetch: func(context.Context) (*kgo.Record, error) {
					steps = append(steps, "fetch")
					fetches++
					if fetches > 1 {
						cancel()
						return nil, ctx.Err()
					}
					return message, nil
				},
				commit: func(_ context.Context, messages ...*kgo.Record) error {
					steps = append(steps, "commit")
					if len(messages) != 1 || !reflect.DeepEqual(messages[0], message) {
						t.Fatal("committed a different record")
					}
					return tc.commitErr
				},
			}
			usecase := &notificationtest.Usecase{HandleFunc: func(_ context.Context, request notification.Request) error {
				steps = append(steps, "handle")
				if request != (notification.Request{RequestID: "req-1", Recipient: "user-1", Content: "hello"}) {
					t.Fatalf("wrong mapping: %+v", request)
				}
				return tc.handleErr
			}}
			err := NewNotificationWorker(reader, usecase, nil).Run(ctx)
			if !reflect.DeepEqual(steps, tc.want) {
				t.Fatalf("steps=%v want=%v", steps, tc.want)
			}
			if tc.name == "success" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("expected failure")
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("lost cause: %v", err)
			}
		})
	}
}

// TestDecodeNotificationRejectsInvalidShape ensures malformed or ambiguous
// messages fail without leaking sensitive JSON fragments into diagnostics.
func TestDecodeNotificationRejectsInvalidShape(t *testing.T) {
	for _, payload := range []string{
		`null`, `[]`, `{}`, `{"request_id":"r","recipient":"u","content":" "}`,
		`{"request_id":"r","recipient":"u","content":"c","extra":"secret"}`,
		`{"request_id":"r","recipient":"u","content":"c"} {}`,
		`{"request_id":"r","recipient":"u","content":123}`,
	} {
		if _, err := decodeNotification([]byte(payload)); err == nil {
			t.Fatalf("accepted %s", payload)
		}
	}
}

// TestNotificationWorkerFetchFailure preserves network failures for lifecycle
// reporting while treating cancellation of an idle fetch as normal shutdown.
func TestNotificationWorkerFetchFailure(t *testing.T) {
	failure := errors.New("broker unavailable")
	for _, stopping := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		reader := readerStub{fetch: func(context.Context) (*kgo.Record, error) {
			if stopping {
				cancel()
				return nil, ctx.Err()
			}
			return nil, failure
		}}
		err := NewNotificationWorker(reader, &notificationtest.Usecase{}, nil).Run(ctx)
		cancel()
		if stopping && err != nil {
			t.Fatal(err)
		}
		if !stopping && !errors.Is(err, failure) {
			t.Fatalf("lost fetch cause: %v", err)
		}
	}
}
