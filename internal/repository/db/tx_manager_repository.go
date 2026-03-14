package db

import "context"

// TxRepository scopes repository access to a single transaction so usecases can run multiple repo calls atomically.
// Extend by adding methods that return transactional repository interfaces.
type TxRepository interface{}

// TxFunc is the function run inside a transaction.
type TxFunc func(ctx context.Context, repos TxRepository) error

// TxManager controls transaction lifecycle for usecases.
type TxManager interface {
	WithTx(ctx context.Context, fn TxFunc) error
}
