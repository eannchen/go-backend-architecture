package r2

import (
	"context"
	"strings"
	"testing"

	repoobjectstorage "github.com/eannchen/go-backend-architecture/internal/repository/external/objectstorage"
)

func TestStoreImplementsObjectStorage(t *testing.T) {
	var _ repoobjectstorage.ObjectStorage = (*Store)(nil)
}

func TestNewRequiresConfigFields(t *testing.T) {
	valid := Config{
		AccountID:       "account-id",
		AccessKeyID:     "access-key-id",
		SecretAccessKey: "secret-access-key",
		Bucket:          "bucket",
		Region:          "auto",
	}

	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{name: "account id", mutate: func(cfg *Config) { cfg.AccountID = " \t " }, wantErr: "account id is required"},
		{name: "access key id", mutate: func(cfg *Config) { cfg.AccessKeyID = "" }, wantErr: "access key id is required"},
		{name: "secret access key", mutate: func(cfg *Config) { cfg.SecretAccessKey = "" }, wantErr: "secret access key is required"},
		{name: "bucket", mutate: func(cfg *Config) { cfg.Bucket = "" }, wantErr: "bucket is required"},
		{name: "region", mutate: func(cfg *Config) { cfg.Region = "" }, wantErr: "region is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := valid
			tt.mutate(&cfg)

			_, err := New(context.Background(), cfg)
			if err == nil {
				t.Fatal("expected config validation error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("New() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}
