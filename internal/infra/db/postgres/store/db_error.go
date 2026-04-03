package store

import (
	"errors"
	"fmt"

	"github.com/eannchen/go-backend-architecture/internal/infra/db/postgres"
	repodb "github.com/eannchen/go-backend-architecture/internal/repository/db"
)

// wrapWriteErr wraps errors from write operations (INSERT/UPDATE/UPSERT) and maps known PG
// constraint violations to repository sentinels. Do not use for SELECT or DELETE paths.
func wrapWriteErr(err error, msg string) error {
	if postgres.IsValueTooLong(err) {
		return fmt.Errorf("%s: %w", msg, errors.Join(repodb.ErrValueTooLong, err))
	}
	if postgres.IsCheckViolation(err) {
		return fmt.Errorf("%s: %w", msg, errors.Join(repodb.ErrDataException, err))
	}
	if postgres.IsUniqueViolation(err) {
		return fmt.Errorf("%s: %w", msg, errors.Join(repodb.ErrDuplicateKey, err))
	}
	if postgres.IsForeignKeyViolation(err) {
		return fmt.Errorf("%s: %w", msg, errors.Join(repodb.ErrForeignKey, err))
	}
	return fmt.Errorf("%s: %w", msg, err)
}

// wrapSelectErr wraps errors from select operations (SELECT) and maps known PG
// Do not use for INSERT/UPDATE/UPSERT paths.
func wrapSelectErr(err error, msg string) error {
	if postgres.IsNoRows(err) {
		return fmt.Errorf("%s: %w", msg, errors.Join(repodb.ErrNotFound, err))
	}
	return fmt.Errorf("%s: %w", msg, err)
}
