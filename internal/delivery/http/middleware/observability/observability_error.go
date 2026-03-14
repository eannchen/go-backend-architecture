package observabilitymw

import (
	"errors"
)

// errorCauseChain builds a single string with the full error chain (top-level + unwrapped causes) for tracing.
func errorCauseChain(err error) string {
	if err == nil {
		return ""
	}
	var b []byte
	for e := err; e != nil; e = errors.Unwrap(e) {
		if len(b) > 0 {
			b = append(b, "; "...)
		}
		b = append(b, e.Error()...)
	}
	return string(b)
}
