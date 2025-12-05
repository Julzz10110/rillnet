package utils

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateID(t *testing.T) {
	id1 := GenerateID("test")
	id2 := GenerateID("test")
	
	if id1 == id2 {
		t.Error("expected different IDs")
	}
	
	if !strings.HasPrefix(id1, "test_") {
		t.Errorf("expected prefix 'test_', got %s", id1)
	}
}

func TestSanitizeString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"normal string", "hello", "hello"},
		{"with control chars", "hello\x00world", "helloworld"},
		{"with newline", "hello\nworld", "hello\nworld"},
		{"with tabs", "hello\tworld", "hello\tworld"},
		{"with whitespace", "  hello  ", "hello"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeString(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeString(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{"short string", "hello", 10, "hello"},
		{"long string", "hello world", 5, "he..."},
		{"very short max", "hello", 2, "he"},
		{"exact length", "hello", 5, "hello"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateString(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("TruncateString(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
			}
		})
	}
}

func TestNormalizeEmail(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"  User@Example.COM  ", "user@example.com"},
		{"test@example.com", "test@example.com"},
		{"  TEST@EXAMPLE.COM  ", "test@example.com"},
	}
	
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := NormalizeEmail(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeEmail(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMaskSensitive(t *testing.T) {
	tests := []struct {
		input        string
		visibleChars int
		expected     string
	}{
		{"password123", 3, "pas********"},
		{"token", 2, "to***"},
		{"short", 10, "*****"},
	}
	
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := MaskSensitive(tt.input, tt.visibleChars)
			if result != tt.expected {
				t.Errorf("MaskSensitive(%q, %d) = %q, want %q", tt.input, tt.visibleChars, result, tt.expected)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{100 * time.Millisecond, "100ms"},
		{2 * time.Second, "2.00s"},
		{2*time.Minute + 30*time.Second, "2m30s"},
		{2*time.Hour + 30*time.Minute, "2h30m"},
	}
	
	for _, tt := range tests {
		t.Run(tt.duration.String(), func(t *testing.T) {
			result := FormatDuration(tt.duration)
			if !strings.Contains(result, tt.expected[:len(tt.expected)-1]) {
				t.Errorf("FormatDuration(%v) = %q, should contain %q", tt.duration, result, tt.expected)
			}
		})
	}
}

func TestIsExpired(t *testing.T) {
	now := time.Now()
	
	if !IsExpired(now.Add(-2*time.Hour), 1*time.Hour) {
		t.Error("expected expired timestamp")
	}
	
	if IsExpired(now.Add(-30*time.Minute), 1*time.Hour) {
		t.Error("expected non-expired timestamp")
	}
}

func TestGenerateStreamID(t *testing.T) {
	id1 := GenerateStreamID()
	id2 := GenerateStreamID()
	
	if id1 == id2 {
		t.Error("expected different stream IDs")
	}
	
	if !strings.HasPrefix(id1, "stream_") {
		t.Errorf("expected prefix 'stream_', got %s", id1)
	}
}

func TestIsEmpty(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"", true},
		{"   ", true},
		{"hello", false},
		{"  hello  ", false},
	}
	
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := IsEmpty(tt.input)
			if result != tt.expected {
				t.Errorf("IsEmpty(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

