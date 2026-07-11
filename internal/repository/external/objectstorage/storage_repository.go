package objectstorage

import (
	"context"
	"time"
)

type PutObjectResult struct {
	ETag     string
	ByteSize int64
}

type ObjectURLSigner interface {
	SignGetObjectURL(ctx context.Context, key string, ttl time.Duration) (string, error)
}

type ObjectStorage interface {
	PutObject(ctx context.Context, key string, body []byte, contentType string) (PutObjectResult, error)
	ObjectExists(ctx context.Context, key string) (bool, error)
	ObjectURLSigner
}
