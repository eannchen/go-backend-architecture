package errutil

import (
	"fmt"
)

// Step annotates a cleanup or lifecycle step error with step context.
func Step(step string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", step, err)
}
