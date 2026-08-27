package observability

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
	appobservability "github.com/eannchen/go-backend-architecture/internal/observability"
)

type rpcError struct {
	original         error
	chain            string
	details          string
	transportCode    string
	transportMessage string
}

func inspectRPCError(err error, code codes.Code) rpcError {
	if err == nil {
		return rpcError{}
	}
	original := originalRPCError(err)
	details := ""
	if appErr, ok := apperr.As(original); ok && len(appErr.Details) > 0 {
		details = appErr.Details.String()
	}
	return rpcError{
		original:         original,
		chain:            appobservability.ErrorCauseChain(original),
		details:          details,
		transportCode:    code.String(),
		transportMessage: status.Convert(err).Message(),
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
