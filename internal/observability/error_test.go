package observability

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrorCauseChain(t *testing.T) {
	root := errors.New("root")
	wrapper := fmt.Errorf("wrapped: %w", root)
	if got := ErrorCauseChain(wrapper); got != "wrapped: root; root" {
		t.Fatalf("ErrorCauseChain() = %q", got)
	}
}

func TestErrorCauseChainIncludesJoinedCauses(t *testing.T) {
	joined := errors.Join(errors.New("first"), errors.New("second"))
	if got := ErrorCauseChain(joined); got != "first\nsecond; first; second" {
		t.Fatalf("ErrorCauseChain() = %q", got)
	}
}
