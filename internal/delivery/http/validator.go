package http

import (
	"github.com/go-playground/validator/v10"

	usecaseHealth "vocynex-api/internal/usecase/health"
)

type requestValidator struct {
	validate *validator.Validate
}

func newRequestValidator() *requestValidator {
	v := validator.New()
	_ = v.RegisterValidation("health_check_mode", validateHealthCheckMode)

	return &requestValidator{
		validate: v,
	}
}

func (v *requestValidator) Validate(i any) error {
	return v.validate.Struct(i)
}

func validateHealthCheckMode(fl validator.FieldLevel) bool {
	_, err := usecaseHealth.ParseCheckMode(fl.Field().String())
	return err == nil
}
