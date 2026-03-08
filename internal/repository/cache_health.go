package repository

import "context"

type CacheHealthStore interface {
	Ping(ctx context.Context) error
}
