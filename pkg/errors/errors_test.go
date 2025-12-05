package errors

import (
	"errors"
	"testing"
)

func TestAppError_Error(t *testing.T) {
	err := NewAppError(ErrCodeInvalidInput, "test error", 400)
	expected := "INVALID_INPUT: test error"
	if err.Error() != expected {
		t.Errorf("Error() = %v, want %v", err.Error(), expected)
	}
}

func TestAppError_WithCause(t *testing.T) {
	originalErr := errors.New("original error")
	err := WrapError(originalErr, ErrCodeInternal, "wrapped error", 500)
	
	if err.Cause != originalErr {
		t.Errorf("Cause = %v, want %v", err.Cause, originalErr)
	}
	
	// Check error message includes cause
	errorMsg := err.Error()
	if !contains(errorMsg, "original error") {
		t.Errorf("Error() should contain cause, got: %v", errorMsg)
	}
}

func TestAppError_WithContext(t *testing.T) {
	err := NewAppError(ErrCodeInvalidInput, "test error", 400)
	err.WithContext("field", "value").WithContext("count", 42)
	
	if err.Context["field"] != "value" {
		t.Errorf("Context[field] = %v, want 'value'", err.Context["field"])
	}
	if err.Context["count"] != 42 {
		t.Errorf("Context[count] = %v, want 42", err.Context["count"])
	}
}

func TestNewInvalidInputError(t *testing.T) {
	err := NewInvalidInputError("invalid input")
	if err.Code != ErrCodeInvalidInput {
		t.Errorf("Code = %v, want %v", err.Code, ErrCodeInvalidInput)
	}
	if err.HTTPStatus != 400 {
		t.Errorf("HTTPStatus = %v, want 400", err.HTTPStatus)
	}
}

func TestNewNotFoundError(t *testing.T) {
	err := NewNotFoundError("stream")
	if err.Code != ErrCodeNotFound {
		t.Errorf("Code = %v, want %v", err.Code, ErrCodeNotFound)
	}
	if err.HTTPStatus != 404 {
		t.Errorf("HTTPStatus = %v, want 404", err.HTTPStatus)
	}
}

func TestIsAppError(t *testing.T) {
	appErr := NewAppError(ErrCodeInvalidInput, "test", 400)
	regularErr := errors.New("regular error")
	
	if !IsAppError(appErr) {
		t.Error("IsAppError() should return true for AppError")
	}
	if IsAppError(regularErr) {
		t.Error("IsAppError() should return false for regular error")
	}
}

func TestGetAppError(t *testing.T) {
	appErr := NewAppError(ErrCodeInvalidInput, "test", 400)
	
	// Direct AppError
	result := GetAppError(appErr)
	if result != appErr {
		t.Errorf("GetAppError() = %v, want %v", result, appErr)
	}
	
	// Wrapped error
	wrapped := WrapError(errors.New("cause"), ErrCodeInternal, "wrapped", 500)
	result = GetAppError(wrapped)
	if result == nil {
		t.Error("GetAppError() should extract AppError from wrapped error")
	}
	
	// Regular error
	regularErr := errors.New("regular error")
	result = GetAppError(regularErr)
	if result != nil {
		t.Error("GetAppError() should return nil for regular error")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

