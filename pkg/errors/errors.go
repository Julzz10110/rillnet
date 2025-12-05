package errors

import (
	"fmt"
	"net/http"
)

// ErrorCode represents application error codes
type ErrorCode string

const (
	ErrCodeInvalidInput     ErrorCode = "INVALID_INPUT"
	ErrCodeNotFound         ErrorCode = "NOT_FOUND"
	ErrCodeUnauthorized     ErrorCode = "UNAUTHORIZED"
	ErrCodeForbidden        ErrorCode = "FORBIDDEN"
	ErrCodeConflict         ErrorCode = "CONFLICT"
	ErrCodeRateLimit        ErrorCode = "RATE_LIMIT_EXCEEDED"
	ErrCodeInternal         ErrorCode = "INTERNAL_ERROR"
	ErrCodeServiceUnavailable ErrorCode = "SERVICE_UNAVAILABLE"
	ErrCodeBadGateway        ErrorCode = "BAD_GATEWAY"
)

// AppError represents an application error with code and context
type AppError struct {
	Code       ErrorCode
	Message    string
	HTTPStatus int
	Cause      error
	Context    map[string]interface{}
}

// Error implements error interface
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error
func (e *AppError) Unwrap() error {
	return e.Cause
}

// WithContext adds context to the error
func (e *AppError) WithContext(key string, value interface{}) *AppError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// NewAppError creates a new application error
func NewAppError(code ErrorCode, message string, httpStatus int) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		HTTPStatus: httpStatus,
		Context:    make(map[string]interface{}),
	}
}

// WrapError wraps an existing error with application error
func WrapError(err error, code ErrorCode, message string, httpStatus int) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		HTTPStatus: httpStatus,
		Cause:      err,
		Context:    make(map[string]interface{}),
	}
}

// Common error constructors
func NewInvalidInputError(message string) *AppError {
	return NewAppError(ErrCodeInvalidInput, message, http.StatusBadRequest)
}

func NewNotFoundError(resource string) *AppError {
	return NewAppError(ErrCodeNotFound, fmt.Sprintf("%s not found", resource), http.StatusNotFound)
}

func NewUnauthorizedError(message string) *AppError {
	return NewAppError(ErrCodeUnauthorized, message, http.StatusUnauthorized)
}

func NewForbiddenError(message string) *AppError {
	return NewAppError(ErrCodeForbidden, message, http.StatusForbidden)
}

func NewConflictError(message string) *AppError {
	return NewAppError(ErrCodeConflict, message, http.StatusConflict)
}

func NewRateLimitError() *AppError {
	return NewAppError(ErrCodeRateLimit, "rate limit exceeded", http.StatusTooManyRequests)
}

func NewInternalError(message string) *AppError {
	return NewAppError(ErrCodeInternal, message, http.StatusInternalServerError)
}

func NewServiceUnavailableError(message string) *AppError {
	return NewAppError(ErrCodeServiceUnavailable, message, http.StatusServiceUnavailable)
}

// IsAppError checks if error is an AppError
func IsAppError(err error) bool {
	_, ok := err.(*AppError)
	return ok
}

// GetAppError extracts AppError from error chain
func GetAppError(err error) *AppError {
	if err == nil {
		return nil
	}
	
	if appErr, ok := err.(*AppError); ok {
		return appErr
	}
	
	// Try to unwrap
	type unwrapper interface {
		Unwrap() error
	}
	
	if u, ok := err.(unwrapper); ok {
		return GetAppError(u.Unwrap())
	}
	
	return nil
}

