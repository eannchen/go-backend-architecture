package otp

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
	"github.com/eannchen/go-backend-architecture/internal/logger/loggertest"
	repodb "github.com/eannchen/go-backend-architecture/internal/repository/db"
	"github.com/eannchen/go-backend-architecture/internal/repository/db/dbtest"
	"github.com/eannchen/go-backend-architecture/internal/repository/external/otp/otptest"
	"github.com/eannchen/go-backend-architecture/internal/repository/kvstore/kvstoretest"
	"github.com/eannchen/go-backend-architecture/internal/usecase/auth"
)

func TestOTPAuthenticatorSendCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		email          string
		storeErr       error
		sendErr        error
		wantCode       apperr.Code
		wantStoreCalls int
		wantSendCalls  int
	}{
		{
			name:     "rejects empty email",
			wantCode: apperr.CodeInvalidArgument,
		},
		{
			name:           "wraps store failure",
			email:          "user@example.com",
			storeErr:       errors.New("redis unavailable"),
			wantCode:       apperr.CodeInternal,
			wantStoreCalls: 1,
		},
		{
			name:           "wraps email failure",
			email:          "user@example.com",
			sendErr:        errors.New("provider unavailable"),
			wantCode:       apperr.CodeInternal,
			wantStoreCalls: 1,
			wantSendCalls:  1,
		},
		{
			name:           "stores hashed code then sends plain code",
			email:          "user@example.com",
			wantStoreCalls: 1,
			wantSendCalls:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			otpRepo := &kvstoretest.OTPRepository{
				StoreFunc: func(context.Context, string, string, time.Duration) error {
					return tt.storeErr
				},
			}
			emailSender := &otptest.EmailSender{
				SendOTPFunc: func(context.Context, string, string) error {
					return tt.sendErr
				},
			}
			uc := NewOTPAuthenticator(
				&loggertest.Logger{},
				nil,
				nil,
				otpRepo,
				emailSender,
				&dbtest.UserRepository{},
				OTPConfig{CodeLength: 6, TTL: 5 * time.Minute},
			)

			err := uc.SendCode(context.Background(), tt.email)

			if tt.wantCode != "" {
				assertAppCode(t, err, tt.wantCode)
			} else if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
			if otpRepo.StoreCalls != tt.wantStoreCalls {
				t.Fatalf("expected store calls %d, got %d", tt.wantStoreCalls, otpRepo.StoreCalls)
			}
			if emailSender.SendOTPCalls != tt.wantSendCalls {
				t.Fatalf("expected send calls %d, got %d", tt.wantSendCalls, emailSender.SendOTPCalls)
			}
			if tt.wantSendCalls == 1 {
				if emailSender.Email != tt.email {
					t.Fatalf("expected email %q, got %q", tt.email, emailSender.Email)
				}
				if len(emailSender.Code) != 6 {
					t.Fatalf("expected 6 digit code, got %q", emailSender.Code)
				}
				if otpRepo.StoreHashedCode != hashCode(emailSender.Code) {
					t.Fatalf("expected stored hash to match sent code")
				}
				if otpRepo.StoreTTL != 5*time.Minute {
					t.Fatalf("expected ttl %v, got %v", 5*time.Minute, otpRepo.StoreTTL)
				}
			}
		})
	}
}

func TestOTPAuthenticatorVerifyCode(t *testing.T) {
	t.Parallel()

	validCode := "123456"
	validHash := hashCode(validCode)

	tests := []struct {
		name             string
		email            string
		code             string
		storedHash       string
		getOTPErr        error
		deleteErr        error
		getUserResult    repodb.User
		getUserErr       error
		createUserResult repodb.User
		createUserErr    error
		wantIdentity     auth.Identity
		wantCode         apperr.Code
		wantGetOTPCalls  int
		wantDeleteCalls  int
		wantGetUserCalls int
		wantCreateCalls  int
		wantWarnCalls    int
	}{
		{
			name:     "rejects empty email",
			code:     validCode,
			wantCode: apperr.CodeInvalidArgument,
		},
		{
			name:     "rejects empty code",
			email:    "user@example.com",
			wantCode: apperr.CodeInvalidArgument,
		},
		{
			name:            "rejects missing otp",
			email:           "user@example.com",
			code:            validCode,
			getOTPErr:       errors.New("not found"),
			wantCode:        apperr.CodeUnauthorized,
			wantGetOTPCalls: 1,
		},
		{
			name:            "rejects mismatched code",
			email:           "user@example.com",
			code:            "000000",
			storedHash:      validHash,
			wantCode:        apperr.CodeUnauthorized,
			wantGetOTPCalls: 1,
		},
		{
			name:             "returns existing user identity",
			email:            "user@example.com",
			code:             validCode,
			storedHash:       validHash,
			getUserResult:    repodb.User{ID: 42, Email: "user@example.com"},
			wantIdentity:     auth.Identity{UserID: 42, Email: "user@example.com", Method: auth.MethodOTP},
			wantGetOTPCalls:  1,
			wantDeleteCalls:  1,
			wantGetUserCalls: 1,
		},
		{
			name:             "creates user when email lookup misses",
			email:            "new@example.com",
			code:             validCode,
			storedHash:       validHash,
			getUserErr:       errors.New("not found"),
			createUserResult: repodb.User{ID: 99, Email: "new@example.com"},
			wantIdentity:     auth.Identity{UserID: 99, Email: "new@example.com", Method: auth.MethodOTP},
			wantGetOTPCalls:  1,
			wantDeleteCalls:  1,
			wantGetUserCalls: 1,
			wantCreateCalls:  1,
		},
		{
			name:             "logs delete failure and continues",
			email:            "user@example.com",
			code:             validCode,
			storedHash:       validHash,
			deleteErr:        errors.New("delete failed"),
			getUserResult:    repodb.User{ID: 7, Email: "user@example.com"},
			wantIdentity:     auth.Identity{UserID: 7, Email: "user@example.com", Method: auth.MethodOTP},
			wantGetOTPCalls:  1,
			wantDeleteCalls:  1,
			wantGetUserCalls: 1,
			wantWarnCalls:    1,
		},
		{
			name:             "maps duplicate create race to conflict",
			email:            "race@example.com",
			code:             validCode,
			storedHash:       validHash,
			getUserErr:       errors.New("not found"),
			createUserErr:    repodb.ErrDuplicateKey,
			wantCode:         apperr.CodeConflict,
			wantGetOTPCalls:  1,
			wantDeleteCalls:  1,
			wantGetUserCalls: 1,
			wantCreateCalls:  1,
		},
		{
			name:             "maps create timeout",
			email:            "slow@example.com",
			code:             validCode,
			storedHash:       validHash,
			getUserErr:       errors.New("not found"),
			createUserErr:    context.DeadlineExceeded,
			wantCode:         apperr.CodeTimeout,
			wantGetOTPCalls:  1,
			wantDeleteCalls:  1,
			wantGetUserCalls: 1,
			wantCreateCalls:  1,
		},
		{
			name:             "wraps create failure",
			email:            "fail@example.com",
			code:             validCode,
			storedHash:       validHash,
			getUserErr:       errors.New("not found"),
			createUserErr:    errors.New("postgres unavailable"),
			wantCode:         apperr.CodeInternal,
			wantGetOTPCalls:  1,
			wantDeleteCalls:  1,
			wantGetUserCalls: 1,
			wantCreateCalls:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			log := &loggertest.Logger{}
			otpRepo := &kvstoretest.OTPRepository{
				GetFunc: func(context.Context, string) (string, error) {
					return tt.storedHash, tt.getOTPErr
				},
				DeleteFunc: func(context.Context, string) error {
					return tt.deleteErr
				},
			}
			userRepo := &dbtest.UserRepository{
				GetByEmailFunc: func(context.Context, string) (repodb.User, error) {
					return tt.getUserResult, tt.getUserErr
				},
				CreateByEmailFunc: func(context.Context, string) (repodb.User, error) {
					return tt.createUserResult, tt.createUserErr
				},
			}
			uc := NewOTPAuthenticator(log, nil, nil, otpRepo, &otptest.EmailSender{}, userRepo, OTPConfig{})

			got, err := uc.VerifyCode(context.Background(), tt.email, tt.code)

			if tt.wantCode != "" {
				assertAppCode(t, err, tt.wantCode)
			} else if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
			if got != tt.wantIdentity {
				t.Fatalf("expected identity %+v, got %+v", tt.wantIdentity, got)
			}
			if otpRepo.GetCalls != tt.wantGetOTPCalls {
				t.Fatalf("expected otp get calls %d, got %d", tt.wantGetOTPCalls, otpRepo.GetCalls)
			}
			if otpRepo.DeleteCalls != tt.wantDeleteCalls {
				t.Fatalf("expected otp delete calls %d, got %d", tt.wantDeleteCalls, otpRepo.DeleteCalls)
			}
			if userRepo.GetByEmailCalls != tt.wantGetUserCalls {
				t.Fatalf("expected user get calls %d, got %d", tt.wantGetUserCalls, userRepo.GetByEmailCalls)
			}
			if userRepo.CreateByEmailCalls != tt.wantCreateCalls {
				t.Fatalf("expected user create calls %d, got %d", tt.wantCreateCalls, userRepo.CreateByEmailCalls)
			}
			if log.WarnCalls != tt.wantWarnCalls {
				t.Fatalf("expected warn calls %d, got %d", tt.wantWarnCalls, log.WarnCalls)
			}
		})
	}
}

func assertAppCode(t *testing.T, err error, want apperr.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected app error %q, got nil", want)
	}
	appErr, ok := apperr.As(err)
	if !ok {
		t.Fatalf("expected app error, got %T", err)
	}
	if appErr.Code != want {
		t.Fatalf("expected code %q, got %q", want, appErr.Code)
	}
}
