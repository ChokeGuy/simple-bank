package validations

import (
	"fmt"
	"net/mail"
	"regexp"
)

var (
	isValidUsername = regexp.MustCompile(`^[a-zA-Z0-9_]+$`).MatchString
	isValidFullName = regexp.MustCompile(`^[a-zA-Z\s]+$`).MatchString
	hasUpper        = regexp.MustCompile(`[A-Z]`).MatchString
	hasLower        = regexp.MustCompile(`[a-z]`).MatchString
	hasNumber       = regexp.MustCompile(`[0-9]`).MatchString
	hasSpecial      = regexp.MustCompile(`[!@#~$%^&*()_+|<>?:{}]`).MatchString
)

func ValidateString(value string, minLength, maxLength int) error {
	strLen := len(value)
	if strLen < minLength || strLen > maxLength {
		return fmt.Errorf("string length must be between %d and %d", minLength, maxLength)
	}
	return nil
}

func ValidateUsername(username string) error {
	if err := ValidateString(username, 3, 100); err != nil {
		return err
	}

	if !isValidUsername(username) {
		return fmt.Errorf("username must contain only letters, digits and underscores")
	}
	return nil
}

func ValidatePassword(password string) error {
	if err := ValidateString(password, 6, 100); err != nil {
		return fmt.Errorf("password length must be at least 6 characters")
	}

	if !hasUpper(password) {
		return fmt.Errorf("password must contain at least one uppercase letter")
	}

	if !hasLower(password) {
		return fmt.Errorf("password must contain at least one lowercase letter")
	}

	if !hasNumber(password) {
		return fmt.Errorf("password must contain at least one number")
	}

	if !hasSpecial(password) {
		return fmt.Errorf("password must contain at least one special character (!@#~$%%^&*()_+|<>?:{})")
	}

	return nil
}

func ValidateEmail(email string) error {
	if err := ValidateString(email, 3, 200); err != nil {
		return err
	}

	if _, err := mail.ParseAddress(email); err != nil {
		return fmt.Errorf("invalid email address format")
	}
	return nil
}

func ValidateFullName(fullname string) error {
	if err := ValidateString(fullname, 3, 200); err != nil {
		return err
	}

	if !isValidFullName(fullname) {
		return fmt.Errorf("fullname must contain only letters or spaces")
	}
	return nil
}
