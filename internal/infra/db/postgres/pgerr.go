package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// PostgreSQL error codes used to map driver errors to repository sentinels.
// See: https://www.postgresql.org/docs/current/errcodes-appendix.html
const (
	CodeUniqueViolation        = "23505"
	CodeCheckViolation         = "23514"
	CodeStringDataTruncation   = "22001"
	CodeNumericValueOutOfRange = "22003"
)

// IsUniqueViolation reports whether err is a Postgres unique_violation (e.g. unique index).
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == CodeUniqueViolation
}

// IsValueTooLong reports whether err is a string_data_right_truncation (VARCHAR overflow).
func IsValueTooLong(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == CodeStringDataTruncation
}

// IsCheckViolation reports whether err is a CHECK constraint failure.
func IsCheckViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == CodeCheckViolation
}
