package server

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v5"
)

// Validator adapts go-playground/validator to echo.Validator, which in v5 is
// `Validate(i any) error`.
type Validator struct {
	validate *validator.Validate
}

func NewValidator() *Validator {
	return &Validator{validate: validator.New(validator.WithRequiredStructEnabled())}
}

func (v *Validator) Validate(i any) error {
	if err := v.validate.Struct(i); err != nil {
		var invalid *validator.InvalidValidationError
		if errors.As(err, &invalid) {
			return echo.NewHTTPError(http.StatusInternalServerError, "validation misconfigured")
		}

		var fieldErrs validator.ValidationErrors
		if errors.As(err, &fieldErrs) {
			messages := make([]string, 0, len(fieldErrs))
			for _, fe := range fieldErrs {
				messages = append(messages, fmt.Sprintf("%s failed the %q rule", fe.Field(), fe.Tag()))
			}
			return echo.NewHTTPError(http.StatusUnprocessableEntity, strings.Join(messages, "; "))
		}

		return echo.NewHTTPError(http.StatusUnprocessableEntity, err.Error())
	}
	return nil
}
