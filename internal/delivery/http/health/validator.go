package health

import (
	"github.com/go-playground/validator/v10"

	usecasehealth "vocynex-api/internal/usecase/health"
)

func RegisterValidation(v *validator.Validate) error {
	return v.RegisterValidation("health_check_mode", validateCheckMode)
}

func validateCheckMode(fl validator.FieldLevel) bool {
	_, err := usecasehealth.ParseCheckMode(fl.Field().String())
	return err == nil
}
