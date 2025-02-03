package validations

import (
	"regexp"

	"github.com/go-playground/validator/v10"
)

// Password validation function
var ValidPassword validator.Func = func(fieldLevel validator.FieldLevel) bool {
	password := fieldLevel.Field().String()
	var (
		hasMinLen  = len(password) >= 6
		hasUpper   = regexp.MustCompile(`[A-Z]`).MatchString(password)
		hasLower   = regexp.MustCompile(`[a-z]`).MatchString(password)
		hasNumber  = regexp.MustCompile(`[0-9]`).MatchString(password)
		hasSpecial = regexp.MustCompile(`[!@#~$%^&*()_+|<>?:{}]`).MatchString(password)
	)
	return hasMinLen && hasUpper && hasLower && hasNumber && hasSpecial
}
