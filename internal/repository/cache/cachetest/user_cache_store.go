package cachetest

import (
	"context"

	repocache "github.com/eannchen/go-backend-architecture/internal/repository/cache"
	repodb "github.com/eannchen/go-backend-architecture/internal/repository/db"
)

// UserCacheStore is a reusable test double for repocache.UserCacheStore.
type UserCacheStore struct {
	GetByIDFunc     func(context.Context, int64) (repodb.User, bool, error)
	GetByIDCalls    int
	GetByIDID       int64
	SetByIDFunc     func(context.Context, int64, repodb.User) error
	SetByIDCalls    int
	SetByIDID       int64
	SetByIDUser     repodb.User
	DeleteByIDFunc  func(context.Context, int64) error
	DeleteByIDCalls int
	DeleteByIDID    int64
}

func (s *UserCacheStore) GetByID(ctx context.Context, id int64) (repodb.User, bool, error) {
	s.GetByIDCalls++
	s.GetByIDID = id
	if s.GetByIDFunc != nil {
		return s.GetByIDFunc(ctx, id)
	}
	return repodb.User{}, false, nil
}

func (s *UserCacheStore) SetByID(ctx context.Context, id int64, user repodb.User) error {
	s.SetByIDCalls++
	s.SetByIDID = id
	s.SetByIDUser = user
	if s.SetByIDFunc != nil {
		return s.SetByIDFunc(ctx, id, user)
	}
	return nil
}

func (s *UserCacheStore) DeleteByID(ctx context.Context, id int64) error {
	s.DeleteByIDCalls++
	s.DeleteByIDID = id
	if s.DeleteByIDFunc != nil {
		return s.DeleteByIDFunc(ctx, id)
	}
	return nil
}

var _ repocache.UserCacheStore = (*UserCacheStore)(nil)
