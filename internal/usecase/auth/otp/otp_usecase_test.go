package otp_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
	domainuser "github.com/eannchen/go-backend-architecture/internal/domain/user"
	usertest "github.com/eannchen/go-backend-architecture/internal/domain/user/usertest"
	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/logger/loggertest"
	"github.com/eannchen/go-backend-architecture/internal/observability"
	"github.com/eannchen/go-backend-architecture/internal/usecase/auth"
	"github.com/eannchen/go-backend-architecture/internal/usecase/auth/otp"
	otptest "github.com/eannchen/go-backend-architecture/internal/usecase/auth/otp/otptest"
)

// TestOTPAuthenticatorSendCode checks that stored codes are hashed, delivery
// failures trigger cleanup, and dependency failures become application errors.
func TestOTPAuthenticatorSendCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		email           string
		storeErr        error
		sendErr         error
		deleteErr       error
		wantCode        apperr.Code
		wantStoreCalls  int
		wantSendCalls   int
		wantDeleteCalls int
		wantWarnCalls   int
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
			name:            "wraps email failure",
			email:           "user@example.com",
			sendErr:         errors.New("provider unavailable"),
			wantCode:        apperr.CodeInternal,
			wantStoreCalls:  1,
			wantSendCalls:   1,
			wantDeleteCalls: 1,
		},
		{
			name:            "logs cleanup failure after email failure",
			email:           "user@example.com",
			sendErr:         errors.New("provider unavailable"),
			deleteErr:       errors.New("redis unavailable"),
			wantCode:        apperr.CodeInternal,
			wantStoreCalls:  1,
			wantSendCalls:   1,
			wantDeleteCalls: 1,
			wantWarnCalls:   1,
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

			otpRepo := &otptest.OTPRepository{
				StoreFunc: func(context.Context, string, string, time.Duration) error {
					return tt.storeErr
				},
				DeleteFunc: func(context.Context, string) error {
					return tt.deleteErr
				},
			}
			emailSender := &otptest.EmailSender{
				SendOTPFunc: func(context.Context, string, string) error {
					return tt.sendErr
				},
			}
			log := &loggertest.Logger{
				WarnFunc: func(context.Context, string, ...logger.Fields) {},
			}
			uc := otp.NewOTPAuthenticator(
				log,
				observability.NoopTracer{},
				observability.NoopMeter{},
				otpRepo,
				emailSender,
				&usertest.Repository{},
				otp.OTPConfig{CodeLength: 6, TTL: 5 * time.Minute},
			)

			err := uc.SendCode(context.Background(), tt.email)

			if tt.wantCode != "" {
				assertAppCode(t, err, tt.wantCode)
			} else if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
			if len(otpRepo.StoreCalls) != tt.wantStoreCalls {
				t.Fatalf("expected store calls %d, got %d", tt.wantStoreCalls, len(otpRepo.StoreCalls))
			}
			if len(emailSender.SendOTPCalls) != tt.wantSendCalls {
				t.Fatalf("expected send calls %d, got %d", tt.wantSendCalls, len(emailSender.SendOTPCalls))
			}
			if len(otpRepo.DeleteCalls) != tt.wantDeleteCalls {
				t.Fatalf("expected delete calls %d, got %d", tt.wantDeleteCalls, len(otpRepo.DeleteCalls))
			}
			if len(log.WarnCalls) != tt.wantWarnCalls {
				t.Fatalf("expected warn calls %d, got %d", tt.wantWarnCalls, len(log.WarnCalls))
			}
			if tt.wantSendCalls == 1 {
				sent := emailSender.SendOTPCalls[0]
				stored := otpRepo.StoreCalls[0]
				if sent.Email != tt.email {
					t.Fatalf("expected email %q, got %q", tt.email, sent.Email)
				}
				if len(sent.Code) != 6 {
					t.Fatalf("expected 6 digit code, got %q", sent.Code)
				}
				if stored.HashedCode != expectedDigest(sent.Code) {
					t.Fatalf("expected stored hash to match sent code")
				}
				if stored.TTL != 5*time.Minute {
					t.Fatalf("expected ttl %v, got %v", 5*time.Minute, stored.TTL)
				}
			}
		})
	}
}

// TestOTPAuthenticatorVerifyCode checks code verification enforces validity and creates the expected authenticated session.
func TestOTPAuthenticatorVerifyCode(t *testing.T) {
	t.Parallel()

	validCode := "123456"
	validHash := expectedDigest(validCode)

	tests := []struct {
		name             string
		email            string
		code             string
		storedHash       string
		consumeOTPErr    error
		getUserResult    domainuser.User
		getUserErr       error
		createUserResult domainuser.User
		createUserErr    error
		wantIdentity     auth.Identity
		wantCode         apperr.Code
		wantConsumeCalls int
		wantGetUserCalls int
		wantCreateCalls  int
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
			name:             "rejects missing otp",
			email:            "user@example.com",
			code:             validCode,
			consumeOTPErr:    otp.ErrOTPNotFound,
			wantCode:         apperr.CodeUnauthorized,
			wantConsumeCalls: 1,
		},
		{
			name:             "rejects mismatched code",
			email:            "user@example.com",
			code:             "000000",
			storedHash:       validHash,
			wantCode:         apperr.CodeUnauthorized,
			wantConsumeCalls: 1,
		},
		{
			name:             "returns existing user identity",
			email:            "user@example.com",
			code:             validCode,
			storedHash:       validHash,
			getUserResult:    domainuser.User{ID: 42, Email: "user@example.com", Status: domainuser.StatusActive},
			wantIdentity:     auth.Identity{UserID: 42, Email: "user@example.com", Method: auth.MethodOTP},
			wantConsumeCalls: 1,
			wantGetUserCalls: 1,
		},
		{
			name:             "creates user when email lookup misses",
			email:            "new@example.com",
			code:             validCode,
			storedHash:       validHash,
			getUserErr:       domainuser.ErrNotFound,
			createUserResult: domainuser.User{ID: 99, Email: "new@example.com", Status: domainuser.StatusActive},
			wantIdentity:     auth.Identity{UserID: 99, Email: "new@example.com", Method: auth.MethodOTP},
			wantConsumeCalls: 1,
			wantGetUserCalls: 1,
			wantCreateCalls:  1,
		},
		{
			name:             "rejects disabled user",
			email:            "disabled@example.com",
			code:             validCode,
			storedHash:       validHash,
			getUserResult:    domainuser.User{ID: 43, Email: "disabled@example.com", Status: domainuser.StatusDisabled},
			wantCode:         apperr.CodeForbidden,
			wantConsumeCalls: 1,
			wantGetUserCalls: 1,
		},
		{
			name:             "returns internal error when OTP consume fails",
			email:            "user@example.com",
			code:             validCode,
			consumeOTPErr:    errors.New("redis unavailable"),
			wantCode:         apperr.CodeInternal,
			wantConsumeCalls: 1,
		},
		{
			name:             "maps duplicate create race to conflict",
			email:            "race@example.com",
			code:             validCode,
			storedHash:       validHash,
			getUserErr:       domainuser.ErrNotFound,
			createUserErr:    domainuser.ErrAlreadyExists,
			wantCode:         apperr.CodeConflict,
			wantConsumeCalls: 1,
			wantGetUserCalls: 1,
			wantCreateCalls:  1,
		},
		{
			name:             "maps create timeout",
			email:            "slow@example.com",
			code:             validCode,
			storedHash:       validHash,
			getUserErr:       domainuser.ErrNotFound,
			createUserErr:    context.DeadlineExceeded,
			wantCode:         apperr.CodeTimeout,
			wantConsumeCalls: 1,
			wantGetUserCalls: 1,
			wantCreateCalls:  1,
		},
		{
			name:             "wraps create failure",
			email:            "fail@example.com",
			code:             validCode,
			storedHash:       validHash,
			getUserErr:       domainuser.ErrNotFound,
			createUserErr:    errors.New("postgres unavailable"),
			wantCode:         apperr.CodeInternal,
			wantConsumeCalls: 1,
			wantGetUserCalls: 1,
			wantCreateCalls:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			log := logger.NoopLogger{}
			otpRepo := &otptest.OTPRepository{
				ConsumeFunc: func(_ context.Context, _ string, candidateHash string) (bool, error) {
					return candidateHash == tt.storedHash, tt.consumeOTPErr
				},
			}
			userRepo := &usertest.Repository{
				GetByEmailFunc: func(context.Context, string) (domainuser.User, error) {
					return tt.getUserResult, tt.getUserErr
				},
				CreateByEmailFunc: func(context.Context, string) (domainuser.User, error) {
					return tt.createUserResult, tt.createUserErr
				},
			}
			uc := otp.NewOTPAuthenticator(log, observability.NoopTracer{}, observability.NoopMeter{}, otpRepo, &otptest.EmailSender{}, userRepo, otp.OTPConfig{})

			got, err := uc.VerifyCode(context.Background(), tt.email, tt.code)

			if tt.wantCode != "" {
				assertAppCode(t, err, tt.wantCode)
			} else if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
			if got != tt.wantIdentity {
				t.Fatalf("expected identity %+v, got %+v", tt.wantIdentity, got)
			}
			if len(otpRepo.ConsumeCalls) != tt.wantConsumeCalls {
				t.Fatalf("expected otp consume calls %d, got %d", tt.wantConsumeCalls, len(otpRepo.ConsumeCalls))
			}
			if len(userRepo.GetByEmailCalls) != tt.wantGetUserCalls {
				t.Fatalf("expected user get calls %d, got %d", tt.wantGetUserCalls, len(userRepo.GetByEmailCalls))
			}
			if len(userRepo.CreateByEmailCalls) != tt.wantCreateCalls {
				t.Fatalf("expected user create calls %d, got %d", tt.wantCreateCalls, len(userRepo.CreateByEmailCalls))
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

// expectedDigest matches the SHA-256 representation required by the storage contract.
func expectedDigest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
