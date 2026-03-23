package otp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
	"github.com/eannchen/go-backend-architecture/internal/logger"
	"github.com/eannchen/go-backend-architecture/internal/observability"
	repodb "github.com/eannchen/go-backend-architecture/internal/repository/db"
	repoexternal "github.com/eannchen/go-backend-architecture/internal/repository/external/otp"
	repokvstore "github.com/eannchen/go-backend-architecture/internal/repository/kvstore"
	"github.com/eannchen/go-backend-architecture/internal/usecase/auth"
)

// OTPAuthenticator handles one-time-password authentication.
// Implementations decide the channel (email, SMS) and code format.
//
// Strategy examples:
//   - Email OTP: sends a numeric code via email.
//   - SMS OTP: sends a code via SMS provider.
type OTPAuthenticator interface {
	SendCode(ctx context.Context, email string) error
	VerifyCode(ctx context.Context, email, code string) (auth.Identity, error)
}

type otpAuthenticator struct {
	log         logger.Logger
	tracer      observability.Tracer
	otpRepo     repokvstore.OTPRepository
	emailSender repoexternal.EmailSender
	userRepo    repodb.UserRepository
	codeLength  int
	ttl         time.Duration
	sendTotal   observability.Counter
	verifyTotal observability.Counter
}

// OTPConfig holds settings for OTP code generation and expiry.
type OTPConfig struct {
	CodeLength int
	TTL        time.Duration
}

// NewOTPAuthenticator creates an email-based OTP authenticator.
func NewOTPAuthenticator(
	log logger.Logger,
	tracer observability.Tracer,
	meter observability.Meter,
	otpRepo repokvstore.OTPRepository,
	emailSender repoexternal.EmailSender,
	userRepo repodb.UserRepository,
	cfg OTPConfig,
) OTPAuthenticator {
	if tracer == nil {
		tracer = observability.NoopTracer{}
	}
	if meter == nil {
		meter = observability.NoopMeter{}
	}
	if cfg.CodeLength <= 0 {
		cfg.CodeLength = 6
	}
	if cfg.TTL <= 0 {
		cfg.TTL = 5 * time.Minute
	}
	return &otpAuthenticator{
		log:         log,
		tracer:      tracer,
		otpRepo:     otpRepo,
		emailSender: emailSender,
		userRepo:    userRepo,
		codeLength:  cfg.CodeLength,
		ttl:         cfg.TTL,
		sendTotal: meter.Counter("auth_otp_send_total",
			observability.MetricOption{Description: "OTP send attempts by outcome.", Unit: "{attempt}"},
		),
		verifyTotal: meter.Counter("auth_otp_verify_total",
			observability.MetricOption{Description: "OTP verify attempts by outcome.", Unit: "{attempt}"},
		),
	}
}

func (a *otpAuthenticator) SendCode(ctx context.Context, email string) (err error) {
	ctx, span := a.tracer.Start(ctx, "usecase", "otp_authenticator.send_code")
	defer func() {
		outcome := "success"
		if err != nil {
			outcome = "error"
		}
		a.sendTotal.Add(ctx, 1, observability.FromPairs("outcome", outcome))
		span.Finish(err)
	}()

	if email == "" {
		return apperr.New(apperr.CodeInvalidArgument, "email is required")
	}

	code, err := generateNumericCode(a.codeLength)
	if err != nil {
		return apperr.Wrap(err, apperr.CodeInternal, "generate otp code")
	}

	hashed := hashCode(code)
	if err := a.otpRepo.Store(ctx, email, hashed, a.ttl); err != nil {
		return apperr.Wrap(err, apperr.CodeInternal, "store otp code")
	}

	if err := a.emailSender.SendOTP(ctx, email, code); err != nil {
		return apperr.Wrap(err, apperr.CodeInternal, "send otp email")
	}

	return nil
}

func (a *otpAuthenticator) VerifyCode(ctx context.Context, email, code string) (identity auth.Identity, err error) {
	ctx, span := a.tracer.Start(ctx, "usecase", "otp_authenticator.verify_code")
	defer func() {
		outcome := "success"
		if err != nil {
			outcome = "error"
		}
		a.verifyTotal.Add(ctx, 1, observability.FromPairs("outcome", outcome))
		span.Finish(err)
	}()

	if email == "" || code == "" {
		return auth.Identity{}, apperr.New(apperr.CodeInvalidArgument, "email and code are required")
	}

	storedHash, err := a.otpRepo.Get(ctx, email)
	if err != nil {
		return auth.Identity{}, apperr.New(apperr.CodeUnauthorized, "invalid or expired otp code")
	}

	if !verifyHash(code, storedHash) {
		return auth.Identity{}, apperr.New(apperr.CodeUnauthorized, "invalid otp code")
	}

	if err := a.otpRepo.Delete(ctx, email); err != nil {
		a.log.Warn(ctx, "otp delete after verify failed", logger.FromPairs("email", email, "error", err))
	}

	user, err := a.userRepo.GetByEmail(ctx, email)
	if err != nil {
		user, err = a.userRepo.CreateByEmail(ctx, email)
		if err != nil {
			if errors.Is(err, repodb.ErrDuplicateKey) {
				// Race: another request created the user; treat as conflict so client can retry or re-login.
				return auth.Identity{}, apperr.New(apperr.CodeConflict, "email already registered")
			}
			if errors.Is(err, context.DeadlineExceeded) {
				return auth.Identity{}, apperr.Wrap(err, apperr.CodeTimeout, "create user timed out")
			}
			return auth.Identity{}, apperr.Wrap(err, apperr.CodeInternal, "find or create user")
		}
	}

	return auth.Identity{
		UserID: user.ID,
		Email:  user.Email,
		Method: auth.MethodOTP,
	}, nil
}

func generateNumericCode(length int) (string, error) {
	max := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(length)), nil)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", fmt.Errorf("crypto/rand: %w", err)
	}
	return fmt.Sprintf("%0*d", length, n), nil
}

func hashCode(code string) string {
	h := sha256.Sum256([]byte(code))
	return hex.EncodeToString(h[:])
}

func verifyHash(code, storedHash string) bool {
	h := sha256.Sum256([]byte(code))
	candidateHex := hex.EncodeToString(h[:])
	return subtle.ConstantTimeCompare([]byte(candidateHex), []byte(storedHash)) == 1
}
