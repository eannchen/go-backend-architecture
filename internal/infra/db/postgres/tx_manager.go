package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/eannchen/go-backend-architecture/internal/observability"
	repodb "github.com/eannchen/go-backend-architecture/internal/repository/db"
)

type TxManager struct {
	pool   *pgxpool.Pool
	tracer observability.Tracer
}

func NewTxManager(pool *pgxpool.Pool, tracer observability.Tracer) *TxManager {
	return &TxManager{pool: pool, tracer: tracer}
}

func (m *TxManager) WithTx(ctx context.Context, fn repodb.TxFunc) error {
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

	repos := newTxRepositories(tx, m.tracer)

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
	tx     pgx.Tx
	tracer observability.Tracer
}

func newTxRepositories(tx pgx.Tx, tracer observability.Tracer) *txRepositories {
	return &txRepositories{tx: tx, tracer: tracer}
}
