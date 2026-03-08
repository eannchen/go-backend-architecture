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

type AccountSummarySearchFilter struct {
	Plan        string
	Status      string
	EmailLike   string
	Limit       uint64
	Offset      uint64
	SortUpdated string
}

// AccountSummaryRepository provides read access for cache-worthy account summary data.
type AccountSummaryRepository interface {
	GetByID(ctx context.Context, id int64) (AccountSummary, error)
	Search(ctx context.Context, filter AccountSummarySearchFilter) ([]AccountSummary, error)
}
