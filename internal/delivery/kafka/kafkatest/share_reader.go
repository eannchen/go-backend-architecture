// Package kafkatest provides configurable doubles for Kafka delivery contracts.
package kafkatest

import (
	"context"

	kafkadelivery "github.com/eannchen/go-backend-architecture/internal/delivery/kafka/handler/notification"
	"github.com/twmb/franz-go/pkg/kgo"
)

// ShareReader is owned by one worker goroutine, like the real reader.
type ShareReader struct {
	FetchRecordsFunc                             func(context.Context) ([]*kgo.Record, error)
	FlushAcknowledgmentsFunc                     func(context.Context) error
	AcceptFunc, ReleaseFunc, RejectFunc          func(*kgo.Record) error
	FetchRecordsCalls, FlushAcknowledgmentsCalls int
	AcceptCalls, ReleaseCalls, RejectCalls       []*kgo.Record
}

func (r *ShareReader) FetchRecords(ctx context.Context) ([]*kgo.Record, error) {
	r.FetchRecordsCalls++
	if r.FetchRecordsFunc == nil {
		panic("unexpected FetchRecords call")
	}
	return r.FetchRecordsFunc(ctx)
}
func (r *ShareReader) FlushAcknowledgments(ctx context.Context) error {
	r.FlushAcknowledgmentsCalls++
	if r.FlushAcknowledgmentsFunc == nil {
		panic("unexpected FlushAcknowledgments call")
	}
	return r.FlushAcknowledgmentsFunc(ctx)
}
func (r *ShareReader) Accept(record *kgo.Record) error {
	r.AcceptCalls = append(r.AcceptCalls, record)
	if r.AcceptFunc == nil {
		panic("unexpected Accept call")
	}
	return r.AcceptFunc(record)
}
func (r *ShareReader) Release(record *kgo.Record) error {
	r.ReleaseCalls = append(r.ReleaseCalls, record)
	if r.ReleaseFunc == nil {
		panic("unexpected Release call")
	}
	return r.ReleaseFunc(record)
}
func (r *ShareReader) Reject(record *kgo.Record) error {
	r.RejectCalls = append(r.RejectCalls, record)
	if r.RejectFunc == nil {
		panic("unexpected Reject call")
	}
	return r.RejectFunc(record)
}

var _ kafkadelivery.ShareMessageReader = (*ShareReader)(nil)
