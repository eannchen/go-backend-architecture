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
	if r.StoreFunc != nil {
		return r.StoreFunc(ctx, email, hashedCode, ttl)
	}
	return nil
}

func (r *OTPRepository) Consume(ctx context.Context, email, candidateHash string) (bool, error) {
	r.ConsumeCalls++
	r.ConsumeEmail = email
	r.ConsumeHash = candidateHash
	if r.ConsumeFunc != nil {
		return r.ConsumeFunc(ctx, email, candidateHash)
	}
	return false, nil
}

func (r *OTPRepository) Delete(ctx context.Context, email string) error {
	r.DeleteCalls++
	r.DeleteEmail = email
	if r.DeleteFunc != nil {
		return r.DeleteFunc(ctx, email)
	}
	return nil
}

var _ repokvstore.OTPRepository = (*OTPRepository)(nil)
