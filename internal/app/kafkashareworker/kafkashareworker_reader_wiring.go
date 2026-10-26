package kafkashareworker

import (
	"context"

	kafkadelivery "github.com/eannchen/go-backend-architecture/internal/delivery/kafka"
	kafkaclient "github.com/eannchen/go-backend-architecture/internal/infra/messaging/kafka"
	"github.com/twmb/franz-go/pkg/kgo"
)

// Adapt concrete client return types here so delivery need not import infra.
type shareReaderAdapter struct{ *kafkaclient.ShareReader }

func (r shareReaderAdapter) FetchBatch(ctx context.Context) (kafkadelivery.ShareWorkBatch, error) {
	batch, err := r.ShareReader.FetchBatch(ctx)
	if err != nil {
		return nil, err
	}
	return shareBatchAdapter{batch}, nil
}

type shareBatchAdapter struct{ *kafkaclient.ShareBatch }

func (b shareBatchAdapter) Accept(record *kgo.Record) error {
	return b.Mark(record, kafkaclient.ShareAccept)
}
func (b shareBatchAdapter) Release(record *kgo.Record) error {
	return b.Mark(record, kafkaclient.ShareRelease)
}
func (b shareBatchAdapter) Reject(record *kgo.Record) error {
	return b.Mark(record, kafkaclient.ShareReject)
}
