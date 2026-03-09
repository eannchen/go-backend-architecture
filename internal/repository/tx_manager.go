package repository

import "context"

// TxRepository scopes repository access to a single transaction so usecases can run multiple repo calls atomically.
type TxRepository interface {
	AccountSummary() AccountSummaryRepository
}

type TxFunc func(ctx context.Context, repos TxRepository) error

// TxManager controls transaction lifecycle for usecases.
type TxManager interface {
	WithTx(ctx context.Context, fn TxFunc) error
}
