// Package kafkatest provides configurable doubles for Kafka delivery contracts.
package kafkatest

import (
	"context"

	kafkadelivery "github.com/eannchen/go-backend-architecture/internal/delivery/kafka"
	"github.com/twmb/franz-go/pkg/kgo"
)

// ShareReader is owned by one worker goroutine, like the real reader.
type ShareReader struct {
	FetchBatchFunc            func(context.Context) (kafkadelivery.ShareWorkBatch, error)
	FlushAcknowledgmentsFunc  func(context.Context) error
	FetchBatchCalls           int
	FlushAcknowledgmentsCalls int
}

func (r *ShareReader) FetchBatch(ctx context.Context) (kafkadelivery.ShareWorkBatch, error) {
	r.FetchBatchCalls++
	if r.FetchBatchFunc == nil {
		panic("unexpected FetchBatch call")
	}
	return r.FetchBatchFunc(ctx)
}
func (r *ShareReader) FlushAcknowledgments(ctx context.Context) error {
	r.FlushAcknowledgmentsCalls++
	if r.FlushAcknowledgmentsFunc == nil {
		panic("unexpected FlushAcknowledgments call")
	}
	return r.FlushAcknowledgmentsFunc(ctx)
}

type ShareBatch struct {
	RecordsFunc                            func() []*kgo.Record
	AcceptFunc                             func(*kgo.Record) error
	ReleaseFunc                            func(*kgo.Record) error
	RejectFunc                             func(*kgo.Record) error
	AcceptCalls, ReleaseCalls, RejectCalls []*kgo.Record
}

func (b *ShareBatch) Records() []*kgo.Record {
	if b.RecordsFunc == nil {
		panic("unexpected Records call")
	}
	return b.RecordsFunc()
}
func (b *ShareBatch) Accept(r *kgo.Record) error {
	b.AcceptCalls = append(b.AcceptCalls, r)
	if b.AcceptFunc == nil {
		panic("unexpected Accept call")
	}
	return b.AcceptFunc(r)
}
func (b *ShareBatch) Release(r *kgo.Record) error {
	b.ReleaseCalls = append(b.ReleaseCalls, r)
	if b.ReleaseFunc == nil {
		panic("unexpected Release call")
	}
	return b.ReleaseFunc(r)
}
func (b *ShareBatch) Reject(r *kgo.Record) error {
	b.RejectCalls = append(b.RejectCalls, r)
	if b.RejectFunc == nil {
		panic("unexpected Reject call")
	}
	return b.RejectFunc(r)
}
