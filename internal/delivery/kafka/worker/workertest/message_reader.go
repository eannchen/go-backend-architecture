package workertest

import (
	"context"

	"github.com/twmb/franz-go/pkg/kgo"
)

// MessageReader is a configurable double for its parent package's MessageReader contract.
type MessageReader struct {
	FetchRecordFunc     func(ctx context.Context) (*kgo.Record, error)
	FetchRecordCalls    []MessageReaderFetchRecordCall
	CommitRecordsFunc   func(ctx context.Context, records ...*kgo.Record) error
	CommitRecordsCalls  []MessageReaderCommitRecordsCall
	AllowRebalanceFunc  func()
	AllowRebalanceCalls []MessageReaderAllowRebalanceCall
}

// MessageReaderFetchRecordCall records one MessageReader.FetchRecord invocation.
type MessageReaderFetchRecordCall struct {
	Context context.Context
}

func (d *MessageReader) FetchRecord(ctx context.Context) (*kgo.Record, error) {
	d.FetchRecordCalls = append(d.FetchRecordCalls, MessageReaderFetchRecordCall{Context: ctx})
	fn := d.FetchRecordFunc
	if fn == nil {
		panic("unexpected MessageReader.FetchRecord call")
	}
	return fn(ctx)
}

// MessageReaderCommitRecordsCall records one MessageReader.CommitRecords invocation.
type MessageReaderCommitRecordsCall struct {
	Context context.Context
	Records []*kgo.Record
}

func (d *MessageReader) CommitRecords(ctx context.Context, records ...*kgo.Record) error {
	d.CommitRecordsCalls = append(d.CommitRecordsCalls, MessageReaderCommitRecordsCall{Context: ctx, Records: append([]*kgo.Record(nil), records...)})
	fn := d.CommitRecordsFunc
	if fn == nil {
		panic("unexpected MessageReader.CommitRecords call")
	}
	return fn(ctx, records...)
}

// MessageReaderAllowRebalanceCall records one MessageReader.AllowRebalance invocation.
type MessageReaderAllowRebalanceCall struct {
}

func (d *MessageReader) AllowRebalance() {
	d.AllowRebalanceCalls = append(d.AllowRebalanceCalls, MessageReaderAllowRebalanceCall{})
	fn := d.AllowRebalanceFunc
	if fn == nil {
		panic("unexpected MessageReader.AllowRebalance call")
	}
	fn()
}
