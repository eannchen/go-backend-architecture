package workertest_test

import (
	"context"
	"testing"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/eannchen/go-backend-architecture/internal/delivery/kafka/worker/workertest"
)

// TestMessageReaderCommitHistory keeps the argument slice stable when the caller
// reuses it, while preserving the original record pointers and call context.
func TestMessageReaderCommitHistory(t *testing.T) {
	reader := &workertest.MessageReader{CommitRecordsFunc: func(context.Context, ...*kgo.Record) error { return nil }}
	first, second := &kgo.Record{Offset: 1}, &kgo.Record{Offset: 2}
	records := []*kgo.Record{first}
	ctx := t.Context()
	if err := reader.CommitRecords(ctx, records...); err != nil {
		t.Fatal(err)
	}
	records[0] = second
	if err := reader.CommitRecords(ctx, records...); err != nil {
		t.Fatal(err)
	}
	if len(reader.CommitRecordsCalls) != 2 {
		t.Fatalf("commit calls = %+v, want two", reader.CommitRecordsCalls)
	}
	for i, record := range []*kgo.Record{first, second} {
		call := reader.CommitRecordsCalls[i]
		if call.Context != ctx || len(call.Records) != 1 || call.Records[0] != record {
			t.Fatalf("commit call %d = %+v", i, call)
		}
	}
}
