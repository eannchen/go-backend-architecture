package store

import (
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	domainuser "github.com/eannchen/go-backend-architecture/internal/domain/user"
)

// TestUserStoreErrors preserves domain outcomes and driver causes so callers can
// handle expected user conflicts without depending on PostgreSQL error types.
func TestUserStoreErrors(t *testing.T) {
	tests := []struct {
		name    string
		wrap    func(error, string) error
		cause   error
		outcome error
	}{
		{"duplicate user", wrapWriteErr, &pgconn.PgError{Code: "23505"}, domainuser.ErrAlreadyExists},
		{"missing user", wrapSelectErr, pgx.ErrNoRows, domainuser.ErrNotFound},
		{"other constraint", wrapWriteErr, &pgconn.PgError{Code: "22001"}, nil},
		{"unexpected read failure", wrapSelectErr, errors.New("connection lost"), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.wrap(tt.cause, "user operation")
			if !errors.Is(err, tt.cause) {
				t.Fatalf("driver cause lost: %v", err)
			}
			if !strings.Contains(err.Error(), "user operation") {
				t.Fatalf("operation context lost: %v", err)
			}
			if tt.outcome != nil && !errors.Is(err, tt.outcome) {
				t.Fatalf("error = %v, want %v", err, tt.outcome)
			}
			if tt.outcome == nil && (errors.Is(err, domainuser.ErrAlreadyExists) || errors.Is(err, domainuser.ErrNotFound)) {
				t.Fatalf("unexpected domain outcome: %v", err)
			}
		})
	}
}
