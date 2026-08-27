package observability

import (
	"errors"

	"google.golang.org/grpc/status"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
	appobservability "github.com/eannchen/go-backend-architecture/internal/observability"
)

type rpcErrorInfo struct {
	original error
	chain    string
	details  string
	code     string
	message  string
}

func inspectRPCError(err error) rpcErrorInfo {
	if err == nil {
		return rpcErrorInfo{}
	}
	original := originalRPCError(err)
	code := ""
	details := ""
	if appErr, ok := apperr.As(original); ok {
		code = string(appErr.Code)
		if len(appErr.Details) > 0 {
			details = appErr.Details.String()
		}
	}
	return rpcErrorInfo{
		original: original,
		chain:    appobservability.ErrorCauseChain(original),
		details:  details,
		code:     code,
		message:  status.Convert(err).Message(),
	}
}

type causalGRPCStatusError interface {
	error
	GRPCStatus() *status.Status
	Unwrap() error
}

func originalRPCError(err error) error {
	var responseErr causalGRPCStatusError
	if errors.As(err, &responseErr) {
		if cause := responseErr.Unwrap(); cause != nil {
			return cause
		}
	}
	return err
}
