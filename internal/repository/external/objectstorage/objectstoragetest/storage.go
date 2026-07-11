package objectstoragetest

import (
	"context"
	"testing"
	"time"

	repoobjectstorage "github.com/eannchen/go-backend-architecture/internal/repository/external/objectstorage"
)

type PutObjectCall struct {
	Key         string
	Body        []byte
	ContentType string
}

type SignGetObjectURLCall struct {
	Key string
	TTL time.Duration
}

type Storage struct {
	T testing.TB

	PutObjectFunc        func(context.Context, string, []byte, string) (repoobjectstorage.PutObjectResult, error)
	ObjectExistsFunc     func(context.Context, string) (bool, error)
	SignGetObjectURLFunc func(context.Context, string, time.Duration) (string, error)

	PutObjectCalls        []PutObjectCall
	ObjectExistsCalls     []string
	SignGetObjectURLCalls []SignGetObjectURLCall
}

var _ repoobjectstorage.ObjectStorage = (*Storage)(nil)

func (s *Storage) PutObject(ctx context.Context, key string, body []byte, contentType string) (repoobjectstorage.PutObjectResult, error) {
	s.PutObjectCalls = append(s.PutObjectCalls, PutObjectCall{
		Key:         key,
		Body:        append([]byte(nil), body...),
		ContentType: contentType,
	})
	if s.PutObjectFunc == nil {
		s.unexpected("PutObject")
		return repoobjectstorage.PutObjectResult{}, nil
	}
	return s.PutObjectFunc(ctx, key, body, contentType)
}

func (s *Storage) ObjectExists(ctx context.Context, key string) (bool, error) {
	s.ObjectExistsCalls = append(s.ObjectExistsCalls, key)
	if s.ObjectExistsFunc == nil {
		return false, nil
	}
	return s.ObjectExistsFunc(ctx, key)
}

func (s *Storage) SignGetObjectURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	s.SignGetObjectURLCalls = append(s.SignGetObjectURLCalls, SignGetObjectURLCall{Key: key, TTL: ttl})
	if s.SignGetObjectURLFunc == nil {
		s.unexpected("SignGetObjectURL")
		return "", nil
	}
	return s.SignGetObjectURLFunc(ctx, key, ttl)
}

type Signer struct {
	T testing.TB

	SignGetObjectURLFunc  func(context.Context, string, time.Duration) (string, error)
	SignGetObjectURLCalls []SignGetObjectURLCall
}

var _ repoobjectstorage.ObjectURLSigner = (*Signer)(nil)

func (s *Signer) SignGetObjectURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	s.SignGetObjectURLCalls = append(s.SignGetObjectURLCalls, SignGetObjectURLCall{Key: key, TTL: ttl})
	if s.SignGetObjectURLFunc == nil {
		s.unexpected("SignGetObjectURL")
		return "", nil
	}
	return s.SignGetObjectURLFunc(ctx, key, ttl)
}

func (s *Storage) unexpected(method string) {
	if s.T != nil {
		s.T.Helper()
		s.T.Fatalf("unexpected ObjectStorage.%s call", method)
	}
}

func (s *Signer) unexpected(method string) {
	if s.T != nil {
		s.T.Helper()
		s.T.Fatalf("unexpected ObjectURLSigner.%s call", method)
	}
}
