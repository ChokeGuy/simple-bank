package validations

import (
	"github.com/ChokeGuy/simple-bank/util"
	"github.com/go-playground/validator/v10"
)

var ValidCurrency validator.Func = func(fieldLevel validator.FieldLevel) bool {
	if currency, ok := fieldLevel.Field().Interface().(string); ok {
		//check if currency is supported
		return util.IsSupportedCurrency(currency)
	}

	return false
}
