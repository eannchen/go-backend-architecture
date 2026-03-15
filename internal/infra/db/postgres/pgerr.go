package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// PostgreSQL error codes used to map driver errors to repository sentinels.
// See: https://www.postgresql.org/docs/current/errcodes-appendix.html
const (
	CodeUniqueViolation = "23505"
)

// IsUniqueViolation reports whether err is a Postgres unique_violation (e.g. unique index).
// Shared by stores in this package when mapping to repodb.ErrDuplicateKey.
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == CodeUniqueViolation
}
