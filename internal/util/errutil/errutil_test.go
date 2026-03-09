package errutil

import (
	"errors"
	"strings"
	"testing"
)

func TestStep(t *testing.T) {
	base := errors.New("close failed")

	err := Step("shutdown cache", base)
	if err == nil {
		t.Fatal("expected wrapped error, got nil")
	}
	if !strings.Contains(err.Error(), "shutdown cache") {
		t.Fatalf("expected step context in error, got %q", err.Error())
	}
	if !errors.Is(err, base) {
		t.Fatal("expected wrapped error to match base error")
	}
}

func TestStep_NilError(t *testing.T) {
	if err := Step("shutdown cache", nil); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestJoin(t *testing.T) {
	base := errors.New("primary failure")
	cleanup := errors.New("cleanup failure")

	err := Join(base, cleanup)
	if err == nil {
		t.Fatal("expected joined error, got nil")
	}
	if !errors.Is(err, base) {
		t.Fatal("expected joined error to include base error")
	}
	if !errors.Is(err, cleanup) {
		t.Fatal("expected joined error to include cleanup error")
	}
}
