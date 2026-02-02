package gateway

import (
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

// CustomValidator implements Echo's Validator interface.
type CustomValidator struct {
	validator *validator.Validate
}

// NewCustomValidator creates a new custom validator.
func NewCustomValidator() *CustomValidator {
	return &CustomValidator{validator: validator.New()}
}

// Validate validates the request body.
func (cv *CustomValidator) Validate(i interface{}) error {
	if err := cv.validator.Struct(i); err != nil {
		// Optionally, you could extract the errors and yield cleaner responses
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return nil
}
