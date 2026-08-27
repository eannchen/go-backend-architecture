package observability

import (
	"errors"
	"strings"
)

// ErrorCauseChain returns an error and all of its wrapped causes as one string.
func ErrorCauseChain(err error) string {
	if err == nil {
		return ""
	}
	var values []string
	appendErrorChain(&values, err, 0)
	return strings.Join(values, "; ")
}

func appendErrorChain(values *[]string, err error, depth int) {
	if err == nil || depth >= 32 {
		return
	}
	*values = append(*values, err.Error())
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		for _, cause := range joined.Unwrap() {
			appendErrorChain(values, cause, depth+1)
		}
		return
	}
	appendErrorChain(values, errors.Unwrap(err), depth+1)
}
