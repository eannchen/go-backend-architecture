package grpcapi

import (
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	rediscachestore "github.com/eannchen/go-backend-architecture/internal/infra/cache/redis/store"
	postgresstore "github.com/eannchen/go-backend-architecture/internal/infra/db/postgres/store"
	rediskvstore "github.com/eannchen/go-backend-architecture/internal/infra/kvstore/redis/store"
	usecasehealth "github.com/eannchen/go-backend-architecture/internal/usecase/health"
)

func (d wiring) buildHealthUsecase(pool *pgxpool.Pool, redisClient *goredis.Client) usecasehealth.Usecase {
	return usecasehealth.New(
		d.tracer,
		d.meter,
		postgresstore.NewDBHealthStore(pool, d.tracer),
		rediscachestore.NewHealthStore(redisClient),
		rediskvstore.NewHealthStore(redisClient),
	)
}
