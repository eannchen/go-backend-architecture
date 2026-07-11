package r2

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"

	repoobjectstorage "github.com/eannchen/go-backend-architecture/internal/repository/external/objectstorage"
)

type Config struct {
	AccountID       string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	Region          string
}

type Store struct {
	client    *s3.Client
	presigner *s3.PresignClient
	bucket    string
}

func New(ctx context.Context, cfg Config) (*Store, error) {
	cfg.AccountID = strings.TrimSpace(cfg.AccountID)
	cfg.AccessKeyID = strings.TrimSpace(cfg.AccessKeyID)
	cfg.SecretAccessKey = strings.TrimSpace(cfg.SecretAccessKey)
	cfg.Bucket = strings.TrimSpace(cfg.Bucket)
	cfg.Region = strings.TrimSpace(cfg.Region)

	if cfg.AccountID == "" {
		return nil, fmt.Errorf("r2 config account id is required")
	}
	if cfg.AccessKeyID == "" {
		return nil, fmt.Errorf("r2 config access key id is required")
	}
	if cfg.SecretAccessKey == "" {
		return nil, fmt.Errorf("r2 config secret access key is required")
	}
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("r2 config bucket is required")
	}
	if cfg.Region == "" {
		return nil, fmt.Errorf("r2 config region is required")
	}

	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.AccountID)
	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("r2 load config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})

	return &Store{
		client:    client,
		presigner: s3.NewPresignClient(client),
		bucket:    cfg.Bucket,
	}, nil
}

func (s *Store) PutObject(ctx context.Context, key string, body []byte, contentType string) (repoobjectstorage.PutObjectResult, error) {
	out, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(body),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return repoobjectstorage.PutObjectResult{}, fmt.Errorf("r2 put object: %w", err)
	}

	var etag string
	if out.ETag != nil {
		etag = *out.ETag
	}

	return repoobjectstorage.PutObjectResult{
		ETag:     etag,
		ByteSize: int64(len(body)),
	}, nil
}

func (s *Store) ObjectExists(ctx context.Context, key string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if _, ok := errors.AsType[*s3types.NotFound](err); ok {
			return false, nil
		}

		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "NotFound" {
			return false, nil
		}
		return false, fmt.Errorf("r2 head object: %w", err)
	}

	return true, nil
}

func (s *Store) SignGetObjectURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	out, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("r2 sign get object: %w", err)
	}

	return out.URL, nil
}

var _ repoobjectstorage.ObjectStorage = (*Store)(nil)
