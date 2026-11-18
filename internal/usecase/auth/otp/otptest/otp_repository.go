package otptest

import (
	"context"
	"time"
)

// OTPRepository is a configurable double for its parent package's OTPRepository contract.
type OTPRepository struct {
	StoreFunc    func(ctx context.Context, email, hashedCode string, ttl time.Duration) error
	StoreCalls   []OTPRepositoryStoreCall
	ConsumeFunc  func(ctx context.Context, email, candidateHash string) (bool, error)
	ConsumeCalls []OTPRepositoryConsumeCall
	DeleteFunc   func(ctx context.Context, email string) error
	DeleteCalls  []OTPRepositoryDeleteCall
}

// OTPRepositoryStoreCall records one OTPRepository.Store invocation.
type OTPRepositoryStoreCall struct {
	Context    context.Context
	Email      string
	HashedCode string
	TTL        time.Duration
}

func (d *OTPRepository) Store(ctx context.Context, email, hashedCode string, ttl time.Duration) error {
	d.StoreCalls = append(d.StoreCalls, OTPRepositoryStoreCall{Context: ctx, Email: email, HashedCode: hashedCode, TTL: ttl})
	fn := d.StoreFunc
	if fn == nil {
		panic("unexpected OTPRepository.Store call")
	}
	return fn(ctx, email, hashedCode, ttl)
}

// OTPRepositoryConsumeCall records one OTPRepository.Consume invocation.
type OTPRepositoryConsumeCall struct {
	Context       context.Context
	Email         string
	CandidateHash string
}

func (d *OTPRepository) Consume(ctx context.Context, email, candidateHash string) (bool, error) {
	d.ConsumeCalls = append(d.ConsumeCalls, OTPRepositoryConsumeCall{Context: ctx, Email: email, CandidateHash: candidateHash})
	fn := d.ConsumeFunc
	if fn == nil {
		panic("unexpected OTPRepository.Consume call")
	}
	return fn(ctx, email, candidateHash)
}

// OTPRepositoryDeleteCall records one OTPRepository.Delete invocation.
type OTPRepositoryDeleteCall struct {
	Context context.Context
	Email   string
}

func (d *OTPRepository) Delete(ctx context.Context, email string) error {
	d.DeleteCalls = append(d.DeleteCalls, OTPRepositoryDeleteCall{Context: ctx, Email: email})
	fn := d.DeleteFunc
	if fn == nil {
		panic("unexpected OTPRepository.Delete call")
	}
	return fn(ctx, email)
}
