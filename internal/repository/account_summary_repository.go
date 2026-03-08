package repository

import "context"

type AccountSummary struct {
	ID          int64
	Email       string
	DisplayName string
	Plan        string
	Status      string
	UpdatedAt   int64
}

// AccountSummaryRepository provides read access for cache-worthy account summary data.
type AccountSummaryRepository interface {
	GetByID(ctx context.Context, id int64) (AccountSummary, error)
}
