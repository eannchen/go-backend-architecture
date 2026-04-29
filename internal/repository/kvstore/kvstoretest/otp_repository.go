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
	GetFunc         func(context.Context, string) (string, error)
	GetCalls        int
	GetEmail        string
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

func (r *OTPRepository) Get(ctx context.Context, email string) (string, error) {
	r.GetCalls++
	r.GetEmail = email
	if r.GetFunc != nil {
		return r.GetFunc(ctx, email)
	}
	return "", nil
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
