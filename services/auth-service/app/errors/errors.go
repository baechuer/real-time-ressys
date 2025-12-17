package errors

import (
	"fmt"
	"net/http"
)

// ErrorCode represents different types of application errors
type ErrorCode string

const (
	ErrCodeNotFound     ErrorCode = "NOT_FOUND"
	ErrCodeInvalidInput ErrorCode = "INVALID_INPUT"
	ErrCodeConflict     ErrorCode = "CONFLICT"
	ErrCodeUnauthorized ErrorCode = "UNAUTHORIZED"
	ErrCodeInternal     ErrorCode = "INTERNAL_ERROR"
)

// AppError represents an application error with code and HTTP status
type AppError struct {
	Code    ErrorCode
	Message string
	Err     error
	Status  int
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// Unwrap returns the underlying error
func (e *AppError) Unwrap() error {
	return e.Err
}

// NewNotFound creates a new "not found" error
func NewNotFound(resource string) *AppError {
	return &AppError{
		Code:    ErrCodeNotFound,
		Message: fmt.Sprintf("%s not found", resource),
		Status:  http.StatusNotFound,
	}
}

// NewInvalidInput creates a new "invalid input" error
func NewInvalidInput(message string) *AppError {
	return &AppError{
		Code:    ErrCodeInvalidInput,
		Message: message,
		Status:  http.StatusBadRequest,
	}
}

// NewConflict creates a new "conflict" error (e.g., duplicate email)
func NewConflict(message string) *AppError {
	return &AppError{
		Code:    ErrCodeConflict,
		Message: message,
		Status:  http.StatusConflict,
	}
}

// NewUnauthorized creates a new "unauthorized" error
func NewUnauthorized(message string) *AppError {
	return &AppError{
		Code:    ErrCodeUnauthorized,
		Message: message,
		Status:  http.StatusUnauthorized,
	}
}

// NewInternal creates a new "internal server" error
func NewInternal(message string) *AppError {
	return &AppError{
		Code:    ErrCodeInternal,
		Message: message,
		Status:  http.StatusInternalServerError,
	}
}

// Wrap wraps an existing error with an AppError
func Wrap(err error, code ErrorCode, message string, status int) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Err:     err,
		Status:  status,
	}
}
