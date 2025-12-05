package validation

import (
	"strings"
	"testing"
)

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{"valid email", "user@example.com", false},
		{"valid email with subdomain", "user@mail.example.com", false},
		{"empty email", "", true},
		{"invalid format", "invalid-email", true},
		{"missing @", "userexample.com", true},
		{"too long", strings.Repeat("a", 250) + "@example.com", true},
		{"valid with plus", "user+tag@example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEmail(tt.email)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateEmail() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateUsername(t *testing.T) {
	tests := []struct {
		name    string
		username string
		wantErr bool
	}{
		{"valid username", "user123", false},
		{"valid with underscore", "user_name", false},
		{"valid with dash", "user-name", false},
		{"too short", "ab", true},
		{"empty", "", true},
		{"too long", strings.Repeat("a", 51), true},
		{"invalid chars", "user name", true},
		{"invalid chars 2", "user@name", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUsername(tt.username)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateUsername() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name    string
		password string
		wantErr bool
	}{
		{"valid password", "password123", false},
		{"minimum length", "pass12", false},
		{"empty", "", true},
		{"too short", "pass", true},
		{"too long", strings.Repeat("a", 129), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePassword() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateStreamID(t *testing.T) {
	tests := []struct {
		name    string
		streamID string
		wantErr bool
	}{
		{"valid stream ID", "stream-123", false},
		{"valid with underscore", "stream_123", false},
		{"empty", "", true},
		{"too long", strings.Repeat("a", 101), true},
		{"invalid chars", "stream 123", true},
		{"invalid chars 2", "stream@123", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStreamID(tt.streamID)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateStreamID() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid http", "http://example.com", false},
		{"valid https", "https://example.com", false},
		{"valid ws", "ws://example.com", false},
		{"valid wss", "wss://example.com", false},
		{"empty", "", true},
		{"invalid scheme", "ftp://example.com", true},
		{"no host", "http://", true},
		{"invalid format", "not-a-url", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateBitrate(t *testing.T) {
	tests := []struct {
		name    string
		bitrate int
		wantErr bool
	}{
		{"valid bitrate", 2500, false},
		{"minimum", 100, false},
		{"maximum", 10000, false},
		{"too low", 50, true},
		{"too high", 15000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBitrate(tt.bitrate)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateBitrate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateQuality(t *testing.T) {
	tests := []struct {
		name    string
		quality string
		wantErr bool
	}{
		{"valid low", "low", false},
		{"valid medium", "medium", false},
		{"valid high", "high", false},
		{"invalid", "ultra", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateQuality(tt.quality)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateQuality() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

