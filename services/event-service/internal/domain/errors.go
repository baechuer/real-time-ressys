package domain

import "fmt"

type ErrCode string

const (
	CodeValidation   ErrCode = "validation_error"
	CodeNotFound     ErrCode = "not_found"
	CodeForbidden    ErrCode = "forbidden"
	CodeInvalidState ErrCode = "invalid_state"
)

type AppError struct {
	Code    ErrCode
	Message string
	Meta    map[string]string
}

func (e *AppError) Error() string {
	if len(e.Meta) == 0 {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("%s: %s (%v)", e.Code, e.Message, e.Meta)
}

func ErrValidation(msg string) error { return &AppError{Code: CodeValidation, Message: msg} }
func ErrValidationMeta(msg string, meta map[string]string) error {
	return &AppError{Code: CodeValidation, Message: msg, Meta: meta}
}
func ErrNotFound(msg string) error     { return &AppError{Code: CodeNotFound, Message: msg} }
func ErrForbidden(msg string) error    { return &AppError{Code: CodeForbidden, Message: msg} }
func ErrInvalidState(msg string) error { return &AppError{Code: CodeInvalidState, Message: msg} }
