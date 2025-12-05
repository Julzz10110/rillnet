package validation

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	// EmailRegex validates email format
	EmailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	
	// StreamIDRegex validates stream ID format
	StreamIDRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	
	// PeerIDRegex validates peer ID format
	PeerIDRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
)

// ValidateEmail validates email address
func ValidateEmail(email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return fmt.Errorf("email is required")
	}
	if len(email) > 254 {
		return fmt.Errorf("email is too long (max 254 characters)")
	}
	if !EmailRegex.MatchString(email) {
		return fmt.Errorf("invalid email format")
	}
	return nil
}

// ValidateUsername validates username
func ValidateUsername(username string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return fmt.Errorf("username is required")
	}
	if len(username) < 3 {
		return fmt.Errorf("username must be at least 3 characters")
	}
	if len(username) > 50 {
		return fmt.Errorf("username is too long (max 50 characters)")
	}
	if !regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString(username) {
		return fmt.Errorf("username contains invalid characters (only letters, numbers, _, - allowed)")
	}
	return nil
}

// ValidatePassword validates password
func ValidatePassword(password string) error {
	if password == "" {
		return fmt.Errorf("password is required")
	}
	if len(password) < 6 {
		return fmt.Errorf("password must be at least 6 characters")
	}
	if len(password) > 128 {
		return fmt.Errorf("password is too long (max 128 characters)")
	}
	return nil
}

// ValidateStreamID validates stream ID
func ValidateStreamID(streamID string) error {
	if streamID == "" {
		return fmt.Errorf("stream ID is required")
	}
	if len(streamID) > 100 {
		return fmt.Errorf("stream ID is too long (max 100 characters)")
	}
	if !StreamIDRegex.MatchString(streamID) {
		return fmt.Errorf("invalid stream ID format")
	}
	return nil
}

// ValidatePeerID validates peer ID
func ValidatePeerID(peerID string) error {
	if peerID == "" {
		return fmt.Errorf("peer ID is required")
	}
	if len(peerID) > 100 {
		return fmt.Errorf("peer ID is too long (max 100 characters)")
	}
	if !PeerIDRegex.MatchString(peerID) {
		return fmt.Errorf("invalid peer ID format")
	}
	return nil
}

// ValidateStreamName validates stream name
func ValidateStreamName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("stream name is required")
	}
	if len(name) < 1 {
		return fmt.Errorf("stream name must be at least 1 character")
	}
	if len(name) > 100 {
		return fmt.Errorf("stream name is too long (max 100 characters)")
	}
	// Check for valid UTF-8
	if !utf8.ValidString(name) {
		return fmt.Errorf("stream name contains invalid characters")
	}
	return nil
}

// ValidateURL validates URL format
func ValidateURL(urlStr string) error {
	if urlStr == "" {
		return fmt.Errorf("URL is required")
	}
	u, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "ws" && u.Scheme != "wss" {
		return fmt.Errorf("invalid URL scheme (must be http, https, ws, or wss)")
	}
	if u.Host == "" {
		return fmt.Errorf("URL must have a host")
	}
	return nil
}

// ValidateBitrate validates bitrate value
func ValidateBitrate(bitrate int) error {
	if bitrate < 100 {
		return fmt.Errorf("bitrate must be at least 100 kbps")
	}
	if bitrate > 10000 {
		return fmt.Errorf("bitrate is too high (max 10000 kbps)")
	}
	return nil
}

// ValidateMaxPeers validates max peers value
func ValidateMaxPeers(maxPeers int) error {
	if maxPeers < 1 {
		return fmt.Errorf("max peers must be at least 1")
	}
	if maxPeers > 1000 {
		return fmt.Errorf("max peers is too high (max 1000)")
	}
	return nil
}

// ValidateQuality validates quality level
func ValidateQuality(quality string) error {
	validQualities := map[string]bool{
		"low":    true,
		"medium": true,
		"high":   true,
	}
	if !validQualities[quality] {
		return fmt.Errorf("invalid quality level (must be low, medium, or high)")
	}
	return nil
}

// ValidateNonEmptyString validates that string is not empty after trimming
func ValidateNonEmptyString(s, fieldName string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return fmt.Errorf("%s is required", fieldName)
	}
	return nil
}

// ValidateStringLength validates string length
func ValidateStringLength(s string, min, max int, fieldName string) error {
	length := utf8.RuneCountInString(s)
	if length < min {
		return fmt.Errorf("%s must be at least %d characters", fieldName, min)
	}
	if length > max {
		return fmt.Errorf("%s is too long (max %d characters)", fieldName, max)
	}
	return nil
}

