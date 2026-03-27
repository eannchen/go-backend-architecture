package db

import "errors"

// Sentinel errors for DB repository outcomes. Infra implementations (e.g. postgres store)
// map vendor errors to these so usecases can errors.Is(err, db.ErrX) and return the right apperr code.
var (
	ErrDuplicateKey  = errors.New("db: duplicate key")
	ErrForeignKey    = errors.New("db: foreign key violation")
	ErrValueTooLong  = errors.New("db: value too long for column")
	ErrDataException = errors.New("db: data exception")
)

// IsInputError reports whether err originated from invalid client input (value too long,
// check constraint violation) rather than a server-side problem.
func IsInputError(err error) bool {
	return errors.Is(err, ErrValueTooLong) || errors.Is(err, ErrDataException) || errors.Is(err, ErrForeignKey)
}
