package httptest

import (
	"testing"

	"github.com/go-playground/validator/v10"
)

// Validator wraps the production validator for transport tests.
type Validator struct {
	validator *validator.Validate
}

func NewValidator(t testing.TB, registrars ...func(*validator.Validate) error) *Validator {
	t.Helper()

	v := validator.New()
	for _, register := range registrars {
		if err := register(v); err != nil {
			t.Fatalf("register validation: %v", err)
		}
	}
	return &Validator{validator: v}
}

func (v *Validator) Validate(value any) error {
	return v.validator.Struct(value)
}
