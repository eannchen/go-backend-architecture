package observability

import (
	"errors"

	"google.golang.org/grpc/status"

	"github.com/eannchen/go-backend-architecture/internal/apperr"
	appobservability "github.com/eannchen/go-backend-architecture/internal/observability"
)

type applicationErrorInfo struct {
	// originalError is the internal Go error recovered from the transport response wrapper.
	originalError error
	// causeChain is the diagnostic unwrap chain and may contain high-cardinality internal text.
	causeChain string
	// diagnosticDetails contains serialized responder details for traces and logs only.
	diagnosticDetails string
	// applicationErrorCode is the transport-independent application code assigned by apperr.
	applicationErrorCode string
	// applicationErrorMessage is the safe gRPC status message associated with applicationErrorCode.
	applicationErrorMessage string
}

func inspectApplicationError(err error) applicationErrorInfo {
	if err == nil {
		return applicationErrorInfo{}
	}
	original := originalRPCError(err)
	applicationErrorCode := ""
	diagnosticDetails := ""
	if appErr, ok := apperr.As(original); ok {
		applicationErrorCode = string(appErr.Code)
		if len(appErr.Details) > 0 {
			diagnosticDetails = appErr.Details.String()
		}
	}
	return applicationErrorInfo{
		originalError:           original,
		causeChain:              appobservability.ErrorCauseChain(original),
		diagnosticDetails:       diagnosticDetails,
		applicationErrorCode:    applicationErrorCode,
		applicationErrorMessage: status.Convert(err).Message(),
	}
}

type causalGRPCStatusError interface {
	error
	GRPCStatus() *status.Status
	Unwrap() error
}

func originalRPCError(err error) error {
	if responseErr, ok := errors.AsType[causalGRPCStatusError](err); ok {
		if cause := responseErr.Unwrap(); cause != nil {
			return cause
		}
	}
	return err
}
