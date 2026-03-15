package db

import "errors"

// Sentinel errors for DB repository outcomes. Infra implementations (e.g. postgres store)
// map vendor errors to these so usecases can errors.Is(err, db.ErrX) and return the right apperr code.
var (
	ErrDuplicateKey = errors.New("db: duplicate key")
)
