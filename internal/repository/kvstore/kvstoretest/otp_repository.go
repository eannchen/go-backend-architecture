package kvstoretest

import (
	"context"
	"time"

	repokvstore "github.com/eannchen/go-backend-architecture/internal/repository/kvstore"
)

// OTPRepository is a reusable test double for repokvstore.OTPRepository.
type OTPRepository struct {
	StoreFunc       func(context.Context, string, string, time.Duration) error
	StoreCalls      int
	StoreEmail      string
	StoreHashedCode string
	StoreTTL        time.Duration
	ConsumeFunc     func(context.Context, string, string) (bool, error)
	ConsumeCalls    int
	ConsumeEmail    string
	ConsumeHash     string
	DeleteFunc      func(context.Context, string) error
	DeleteCalls     int
	DeleteEmail     string
}

func (r *OTPRepository) Store(ctx context.Context, email, hashedCode string, ttl time.Duration) error {
	r.StoreCalls++
	r.StoreEmail = email
	r.StoreHashedCode = hashedCode
	r.StoreTTL = ttl
	if r.StoreFunc == nil {
		panic("unexpected OTPRepository.Store call")
	}
	return r.StoreFunc(ctx, email, hashedCode, ttl)
}

func (r *OTPRepository) Consume(ctx context.Context, email, candidateHash string) (bool, error) {
	r.ConsumeCalls++
	r.ConsumeEmail = email
	r.ConsumeHash = candidateHash
	if r.ConsumeFunc == nil {
		panic("unexpected OTPRepository.Consume call")
	}
	return r.ConsumeFunc(ctx, email, candidateHash)
}

func (r *OTPRepository) Delete(ctx context.Context, email string) error {
	r.DeleteCalls++
	r.DeleteEmail = email
	if r.DeleteFunc == nil {
		panic("unexpected OTPRepository.Delete call")
	}
	return r.DeleteFunc(ctx, email)
}

var _ repokvstore.OTPRepository = (*OTPRepository)(nil)
