package workertest

import (
	"context"

	"github.com/twmb/franz-go/pkg/kgo"
)

// ShareMessageReader is a configurable double for its parent package's ShareMessageReader contract.
type ShareMessageReader struct {
	FetchRecordsFunc          func(ctx context.Context) ([]*kgo.Record, error)
	FetchRecordsCalls         []ShareMessageReaderFetchRecordsCall
	AcceptFunc                func(record *kgo.Record) error
	AcceptCalls               []ShareMessageReaderAcceptCall
	ReleaseFunc               func(record *kgo.Record) error
	ReleaseCalls              []ShareMessageReaderReleaseCall
	RejectFunc                func(record *kgo.Record) error
	RejectCalls               []ShareMessageReaderRejectCall
	FlushAcknowledgmentsFunc  func(ctx context.Context) error
	FlushAcknowledgmentsCalls []ShareMessageReaderFlushAcknowledgmentsCall
}

// ShareMessageReaderFetchRecordsCall records one ShareMessageReader.FetchRecords invocation.
type ShareMessageReaderFetchRecordsCall struct {
	Context context.Context
}

func (d *ShareMessageReader) FetchRecords(ctx context.Context) ([]*kgo.Record, error) {
	d.FetchRecordsCalls = append(d.FetchRecordsCalls, ShareMessageReaderFetchRecordsCall{Context: ctx})
	fn := d.FetchRecordsFunc
	if fn == nil {
		panic("unexpected ShareMessageReader.FetchRecords call")
	}
	return fn(ctx)
}

// ShareMessageReaderAcceptCall records one ShareMessageReader.Accept invocation.
type ShareMessageReaderAcceptCall struct {
	Record *kgo.Record
}

func (d *ShareMessageReader) Accept(record *kgo.Record) error {
	d.AcceptCalls = append(d.AcceptCalls, ShareMessageReaderAcceptCall{Record: record})
	fn := d.AcceptFunc
	if fn == nil {
		panic("unexpected ShareMessageReader.Accept call")
	}
	return fn(record)
}

// ShareMessageReaderReleaseCall records one ShareMessageReader.Release invocation.
type ShareMessageReaderReleaseCall struct {
	Record *kgo.Record
}

func (d *ShareMessageReader) Release(record *kgo.Record) error {
	d.ReleaseCalls = append(d.ReleaseCalls, ShareMessageReaderReleaseCall{Record: record})
	fn := d.ReleaseFunc
	if fn == nil {
		panic("unexpected ShareMessageReader.Release call")
	}
	return fn(record)
}

// ShareMessageReaderRejectCall records one ShareMessageReader.Reject invocation.
type ShareMessageReaderRejectCall struct {
	Record *kgo.Record
}

func (d *ShareMessageReader) Reject(record *kgo.Record) error {
	d.RejectCalls = append(d.RejectCalls, ShareMessageReaderRejectCall{Record: record})
	fn := d.RejectFunc
	if fn == nil {
		panic("unexpected ShareMessageReader.Reject call")
	}
	return fn(record)
}

// ShareMessageReaderFlushAcknowledgmentsCall records one ShareMessageReader.FlushAcknowledgments invocation.
type ShareMessageReaderFlushAcknowledgmentsCall struct {
	Context context.Context
}

func (d *ShareMessageReader) FlushAcknowledgments(ctx context.Context) error {
	d.FlushAcknowledgmentsCalls = append(d.FlushAcknowledgmentsCalls, ShareMessageReaderFlushAcknowledgmentsCall{Context: ctx})
	fn := d.FlushAcknowledgmentsFunc
	if fn == nil {
		panic("unexpected ShareMessageReader.FlushAcknowledgments call")
	}
	return fn(ctx)
}
