// Package workertest provides configurable reader doubles for delivery workers.
// Reader calls are owned by the worker's Run goroutine. Inspect observations
// after Run finishes; use callback channels to coordinate in-flight assertions.
package workertest

import (
	"context"

	"github.com/twmb/franz-go/pkg/kgo"
)

type MessageReader struct {
	FetchRecordFunc     func(context.Context) (*kgo.Record, error)
	FetchRecordCalls    int
	CommitRecordsFunc   func(context.Context, ...*kgo.Record) error
	CommitRecordsCalls  [][]*kgo.Record
	AllowRebalanceFunc  func()
	AllowRebalanceCalls int
}

func (r *MessageReader) FetchRecord(ctx context.Context) (*kgo.Record, error) {
	r.FetchRecordCalls++
	if r.FetchRecordFunc == nil {
		panic("unexpected MessageReader.FetchRecord")
	}
	return r.FetchRecordFunc(ctx)
}
func (r *MessageReader) CommitRecords(ctx context.Context, records ...*kgo.Record) error {
	r.CommitRecordsCalls = append(r.CommitRecordsCalls, append([]*kgo.Record(nil), records...))
	if r.CommitRecordsFunc == nil {
		panic("unexpected MessageReader.CommitRecords")
	}
	return r.CommitRecordsFunc(ctx, records...)
}
func (r *MessageReader) AllowRebalance() {
	r.AllowRebalanceCalls++
	if r.AllowRebalanceFunc == nil {
		panic("unexpected MessageReader.AllowRebalance")
	}
	r.AllowRebalanceFunc()
}

type ShareMessageReader struct {
	FetchRecordsFunc          func(context.Context) ([]*kgo.Record, error)
	FetchRecordsCalls         int
	AcceptFunc                func(*kgo.Record) error
	AcceptCalls               []*kgo.Record
	ReleaseFunc               func(*kgo.Record) error
	ReleaseCalls              []*kgo.Record
	RejectFunc                func(*kgo.Record) error
	RejectCalls               []*kgo.Record
	FlushAcknowledgmentsFunc  func(context.Context) error
	FlushAcknowledgmentsCalls int
}

func (r *ShareMessageReader) FetchRecords(ctx context.Context) ([]*kgo.Record, error) {
	r.FetchRecordsCalls++
	if r.FetchRecordsFunc == nil {
		panic("unexpected ShareMessageReader.FetchRecords")
	}
	return r.FetchRecordsFunc(ctx)
}
func (r *ShareMessageReader) Accept(record *kgo.Record) error {
	r.AcceptCalls = append(r.AcceptCalls, record)
	if r.AcceptFunc == nil {
		panic("unexpected ShareMessageReader.Accept")
	}
	return r.AcceptFunc(record)
}
func (r *ShareMessageReader) Release(record *kgo.Record) error {
	r.ReleaseCalls = append(r.ReleaseCalls, record)
	if r.ReleaseFunc == nil {
		panic("unexpected ShareMessageReader.Release")
	}
	return r.ReleaseFunc(record)
}
func (r *ShareMessageReader) Reject(record *kgo.Record) error {
	r.RejectCalls = append(r.RejectCalls, record)
	if r.RejectFunc == nil {
		panic("unexpected ShareMessageReader.Reject")
	}
	return r.RejectFunc(record)
}
func (r *ShareMessageReader) FlushAcknowledgments(ctx context.Context) error {
	r.FlushAcknowledgmentsCalls++
	if r.FlushAcknowledgmentsFunc == nil {
		panic("unexpected ShareMessageReader.FlushAcknowledgments")
	}
	return r.FlushAcknowledgmentsFunc(ctx)
}
