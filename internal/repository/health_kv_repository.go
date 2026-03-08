package repository

import "context"

type KVHealthStore interface {
	Ping(ctx context.Context) error
}
