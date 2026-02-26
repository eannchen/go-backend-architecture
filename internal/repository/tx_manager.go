package repository

import "context"

// TxRepository exposes repository set bound to one DB transaction.
//
// As more repositories are added, expose them here (User(), Order(), etc.).
type TxRepository interface {
	Runtime() RuntimeRepository
}

type TxFunc func(ctx context.Context, repos TxRepository) error

// TxManager controls transaction lifecycle for usecases.
type TxManager interface {
	WithTx(ctx context.Context, fn TxFunc) error
}
