package http

import (
	"fmt"

	"github.com/go-playground/validator/v10"
)

type ValidationRegistrar func(v *validator.Validate) error

type requestValidator struct {
	validate *validator.Validate
}

func newRequestValidator(registrars ...ValidationRegistrar) (*requestValidator, error) {
	v := validator.New()
	for _, register := range registrars {
		if register == nil {
			continue
		}
		if err := register(v); err != nil {
			return nil, fmt.Errorf("register validator: %w", err)
		}
	}

	return &requestValidator{
		validate: v,
	}, nil
}

func (v *requestValidator) Validate(i any) error {
	return v.validate.Struct(i)
}
