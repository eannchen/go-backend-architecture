package store

import (
	"errors"
	"fmt"

	domainuser "github.com/eannchen/go-backend-architecture/internal/domain/user"
	"github.com/eannchen/go-backend-architecture/internal/infra/db/postgres"
)

// wrapWriteErr identifies user uniqueness conflicts while preserving the driver
// error. Other database failures remain infrastructure errors.
func wrapWriteErr(err error, msg string) error {
	if postgres.IsUniqueViolation(err) {
		return fmt.Errorf("%s: %w", msg, errors.Join(domainuser.ErrAlreadyExists, err))
	}
	return fmt.Errorf("%s: %w", msg, err)
}

// wrapSelectErr maps a missing row to the domain’s missing-user outcome.
func wrapSelectErr(err error, msg string) error {
	if postgres.IsNoRows(err) {
		return fmt.Errorf("%s: %w", msg, errors.Join(domainuser.ErrNotFound, err))
	}
	return fmt.Errorf("%s: %w", msg, err)
}
