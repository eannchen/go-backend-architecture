package errutil

import (
	"errors"
	"fmt"
)

// Step annotates a cleanup or lifecycle step error with step context.
func Step(step string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", step, err)
}

// Join combines the base error with optional cleanup errors.
func Join(base error, cleanupErrs ...error) error {
	all := make([]error, 0, len(cleanupErrs)+1)
	all = append(all, base)
	all = append(all, cleanupErrs...)
	return errors.Join(all...)
}
