package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"vocynex-api/internal/infra/db/postgres/repos"
	"vocynex-api/internal/repository"
)

type TxManager struct {
	pool *pgxpool.Pool
}

func NewTxManager(pool *pgxpool.Pool) *TxManager {
	return &TxManager{pool: pool}
}

func (m *TxManager) WithTx(ctx context.Context, fn repository.TxFunc) error {
	tx, err := m.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	repos := newTxRepositories(tx)

	if err := fn(ctx, repos); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	committed = true
	return nil
}

type txRepositories struct {
	tx      pgx.Tx
	runtime repository.RuntimeRepository
}

func newTxRepositories(tx pgx.Tx) *txRepositories {
	return &txRepositories{tx: tx}
}

func (r *txRepositories) Runtime() repository.RuntimeRepository {
	if r.runtime == nil {
		r.runtime = repos.NewRuntimeRepository(r.tx)
	}
	return r.runtime
}
