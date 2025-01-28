package validations

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// CustomErrorMessage maps field names and validation tags to more user-friendly error messages.
var CustomErrorMessage = map[string]map[string]string{
	"Owner": {
		"required": "Owner field is required.",
	},
	"Currency": {
		"required": "Currency field is required.",
		"currency": "Invalid currency format.",
	},
}

// FormatValidationError formats validator errors into a more readable format.
func FormatValidationError(err error) string {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		var errorMessages []string
		for _, fieldErr := range ve {
			field := fieldErr.Field()
			tag := fieldErr.Tag()
			if msg, ok := CustomErrorMessage[field][tag]; ok {
				errorMessages = append(errorMessages, msg)
			} else {
				errorMessages = append(errorMessages, fmt.Sprintf("Field '%s' failed validation on the '%s' rule.", field, tag))
			}
		}
		return fmt.Sprintf("Validation errors: %v", errorMessages)
	}
	return "Invalid input."
}

// HandleValidationError handles validation errors and sends a custom response to the client.
func HandleValidationError(ctx *gin.Context, err error) {
	// You can format the error message as needed
	errorMessage := FormatValidationError(err)
	// Send custom response with status code and error message
	ctx.JSON(http.StatusBadRequest, gin.H{
		"error": errorMessage,
	})
}
