package errors

import "fmt"

// ErrorCode represents different error types
type ErrorCode string

const (
	ErrCodeInvalidInput    ErrorCode = "INVALID_INPUT"
	ErrCodeNotFound        ErrorCode = "NOT_FOUND"
	ErrCodeInternal        ErrorCode = "INTERNAL_ERROR"
	ErrCodeEmailProvider   ErrorCode = "EMAIL_PROVIDER_ERROR"
	ErrCodeRetryable       ErrorCode = "RETRYABLE_ERROR"
	ErrCodePermanentFailure ErrorCode = "PERMANENT_FAILURE"
)

// AppError represents an application error
type AppError struct {
	Code    ErrorCode
	Message string
	Err     error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// NewInvalidInput creates a new invalid input error
func NewInvalidInput(message string) *AppError {
	return &AppError{
		Code:    ErrCodeInvalidInput,
		Message: message,
	}
}

// NewNotFound creates a new not found error
func NewNotFound(message string) *AppError {
	return &AppError{
		Code:    ErrCodeNotFound,
		Message: message,
	}
}

// NewInternal creates a new internal error
func NewInternal(message string) *AppError {
	return &AppError{
		Code:    ErrCodeInternal,
		Message: message,
	}
}

// NewEmailProviderError creates a new email provider error
func NewEmailProviderError(message string, err error) *AppError {
	return &AppError{
		Code:    ErrCodeEmailProvider,
		Message: message,
		Err:     err,
	}
}

// NewRetryableError creates a retryable error
func NewRetryableError(message string, err error) *AppError {
	return &AppError{
		Code:    ErrCodeRetryable,
		Message: message,
		Err:     err,
	}
}

// NewPermanentFailure creates a permanent failure error
func NewPermanentFailure(message string, err error) *AppError {
	return &AppError{
		Code:    ErrCodePermanentFailure,
		Message: message,
		Err:     err,
	}
}

