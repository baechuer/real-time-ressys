package main

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/baechuer/real-time-ressys/services/auth-service/app/errors"
	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

func init() {
	validate = validator.New()

	// Register custom validators
	validate.RegisterValidation("password_strength", validatePasswordStrength)
	validate.RegisterValidation("username_format", validateUsernameFormat)
}

// validatePasswordStrength is a custom validator that checks if password has:
// - At least one uppercase letter
// - At least one lowercase letter
// - At least one number
func validatePasswordStrength(fl validator.FieldLevel) bool {
	password := fl.Field().String()

	hasUpper := false
	hasLower := false
	hasNumber := false

	for _, char := range password {
		if unicode.IsUpper(char) {
			hasUpper = true
		}
		if unicode.IsLower(char) {
			hasLower = true
		}
		if unicode.IsNumber(char) {
			hasNumber = true
		}

		// Early exit if all conditions are met
		if hasUpper && hasLower && hasNumber {
			return true
		}
	}

	return hasUpper && hasLower && hasNumber
}

// validateUsernameFormat checks if username contains only alphanumeric characters and underscores
func validateUsernameFormat(fl validator.FieldLevel) bool {
	username := fl.Field().String()

	if len(username) == 0 {
		return false
	}

	for _, char := range username {
		if !unicode.IsLetter(char) && !unicode.IsNumber(char) && char != '_' {
			return false
		}
	}

	return true
}

// validateRequest validates a request DTO and returns formatted error
func validateRequest(req interface{}) *errors.AppError {
	if err := validate.Struct(req); err != nil {
		return formatValidationErrors(err)
	}
	return nil
}

// formatValidationErrors formats validator errors into user-friendly messages
func formatValidationErrors(err error) *errors.AppError {
	var messages []string

	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		for _, fieldError := range validationErrors {
			msg := formatFieldError(fieldError)
			messages = append(messages, msg)
		}
	} else {
		return errors.NewInvalidInput(err.Error())
	}

	return errors.NewInvalidInput(strings.Join(messages, "; "))
}

// formatFieldError formats a single field validation error
func formatFieldError(fe validator.FieldError) string {
	field := fe.Field()
	tag := fe.Tag()

	switch tag {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "email":
		return fmt.Sprintf("%s must be a valid email address", field)
	case "min":
		return fmt.Sprintf("%s must be at least %s characters", field, fe.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s characters", field, fe.Param())
	case "password_strength":
		return fmt.Sprintf("%s must contain at least one uppercase letter, one lowercase letter, and one number", field)
	case "username_format":
		return fmt.Sprintf("%s can only contain letters, numbers, and underscores", field)
	default:
		return fmt.Sprintf("%s is invalid", field)
	}
}

// sanitizeInput sanitizes user input by trimming whitespace and removing control characters
// maxLength: maximum length in runes (0 = no limit)
// preserveSpecialChars: if true, preserves special characters (for passwords)
func sanitizeInput(input string, maxLength int, preserveSpecialChars bool) string {
	// Trim leading and trailing whitespace
	input = strings.TrimSpace(input)

	// Remove null bytes (always remove these)
	input = strings.ReplaceAll(input, "\x00", "")

	// If preserving special chars (for passwords), only trim and limit length
	if preserveSpecialChars {
		if maxLength > 0 && utf8.RuneCountInString(input) > maxLength {
			runes := []rune(input)
			input = string(runes[:maxLength])
		}
		return input
	}

	// For non-password fields, remove control characters (except newline and tab)
	var builder strings.Builder
	for _, r := range input {
		// Allow printable characters, newline, and tab
		if unicode.IsPrint(r) || r == '\n' || r == '\t' {
			builder.WriteRune(r)
		}
	}
	input = builder.String()

	// Limit length if specified
	if maxLength > 0 && utf8.RuneCountInString(input) > maxLength {
		runes := []rune(input)
		input = string(runes[:maxLength])
	}

	return input
}

// sanitizeEmail sanitizes email input (trims and normalizes)
func sanitizeEmail(email string, maxLength int) string {
	email = sanitizeInput(email, maxLength, false)
	// Convert to lowercase (email addresses are case-insensitive)
	email = strings.ToLower(email)
	return email
}

// sanitizeUsername sanitizes username input (trims, removes invalid characters)
func sanitizeUsername(username string, maxLength int) string {
	username = sanitizeInput(username, maxLength, false)
	// Remove any characters that aren't alphanumeric or underscore
	var builder strings.Builder
	for _, r := range username {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_' {
			builder.WriteRune(r)
		}
	}
	username = builder.String()

	// Limit length again after filtering
	if maxLength > 0 && utf8.RuneCountInString(username) > maxLength {
		runes := []rune(username)
		username = string(runes[:maxLength])
	}

	return username
}
