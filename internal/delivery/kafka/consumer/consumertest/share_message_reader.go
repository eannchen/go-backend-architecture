package consumertest

import (
	"context"

	"github.com/twmb/franz-go/pkg/kgo"
)

// ShareMessageReader is a configurable double for its parent package's ShareMessageReader contract.
type ShareMessageReader struct {
	FetchRecordsFunc    func(ctx context.Context) ([]*kgo.Record, error)
	FetchRecordsCalls   []ShareMessageReaderFetchRecordsCall
	AcceptFunc          func(record *kgo.Record) error
	AcceptCalls         []ShareMessageReaderAcceptCall
	ReleaseFunc         func(record *kgo.Record) error
	ReleaseCalls        []ShareMessageReaderReleaseCall
	RejectFunc          func(record *kgo.Record) error
	RejectCalls         []ShareMessageReaderRejectCall
	RenewRemainingFunc  func() error
	RenewRemainingCalls []ShareMessageReaderRenewRemainingCall
	FatalErrorsFunc     func() <-chan error
	FatalErrorsCalls    []ShareMessageReaderFatalErrorsCall
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

// ShareMessageReaderRenewRemainingCall records one renewal request.
type ShareMessageReaderRenewRemainingCall struct{}

func (d *ShareMessageReader) RenewRemaining() error {
	d.RenewRemainingCalls = append(d.RenewRemainingCalls, ShareMessageReaderRenewRemainingCall{})
	fn := d.RenewRemainingFunc
	if fn == nil {
		panic("unexpected ShareMessageReader.RenewRemaining call")
	}
	return fn()
}

// ShareMessageReaderFatalErrorsCall records one ShareMessageReader.FatalErrors invocation.
type ShareMessageReaderFatalErrorsCall struct{}

func (d *ShareMessageReader) FatalErrors() <-chan error {
	d.FatalErrorsCalls = append(d.FatalErrorsCalls, ShareMessageReaderFatalErrorsCall{})
	fn := d.FatalErrorsFunc
	if fn == nil {
		panic("unexpected ShareMessageReader.FatalErrors call")
	}
	return fn()
}
