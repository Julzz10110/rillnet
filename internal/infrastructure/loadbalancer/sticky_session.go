package loadbalancer

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
)

// StickySessionManager manages sticky sessions for load balancing
type StickySessionManager struct {
	secretKey []byte
	cookieName string
	maxAge     int
}

// NewStickySessionManager creates a new sticky session manager
func NewStickySessionManager(secretKey string, cookieName string, maxAge int) *StickySessionManager {
	return &StickySessionManager{
		secretKey:  []byte(secretKey),
		cookieName: cookieName,
		maxAge:     maxAge,
	}
}

// GetSessionID gets or creates a session ID for the request
func (s *StickySessionManager) GetSessionID(r *http.Request) string {
	// Try to get from cookie
	cookie, err := r.Cookie(s.cookieName)
	if err == nil && cookie.Value != "" {
		// Validate cookie signature
		if s.validateCookie(cookie.Value) {
			return s.extractSessionID(cookie.Value)
		}
	}

	// Generate new session ID
	return s.generateSessionID(r)
}

// SetSessionCookie sets the session cookie in the response
func (s *StickySessionManager) SetSessionCookie(w http.ResponseWriter, sessionID string) {
	signedValue := s.signSessionID(sessionID)
	cookie := &http.Cookie{
		Name:     s.cookieName,
		Value:    signedValue,
		Path:     "/",
		MaxAge:   s.maxAge,
		HttpOnly: true,
		Secure:   true, // Use HTTPS in production
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, cookie)
}

// generateSessionID generates a session ID based on request
func (s *StickySessionManager) generateSessionID(r *http.Request) string {
	// Use IP address and User-Agent for consistent hashing
	ip := s.getClientIP(r)
	ua := r.Header.Get("User-Agent")
	
	data := fmt.Sprintf("%s:%s", ip, ua)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:16]) // Use first 16 bytes
}

// signSessionID signs a session ID with HMAC
func (s *StickySessionManager) signSessionID(sessionID string) string {
	mac := hmac.New(sha256.New, s.secretKey)
	mac.Write([]byte(sessionID))
	signature := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("%s.%s", sessionID, signature)
}

// validateCookie validates the cookie signature
func (s *StickySessionManager) validateCookie(cookieValue string) bool {
	parts := strings.Split(cookieValue, ".")
	if len(parts) != 2 {
		return false
	}

	sessionID := parts[0]
	expectedSignature := s.signSessionID(sessionID)
	return hmac.Equal([]byte(cookieValue), []byte(expectedSignature))
}

// extractSessionID extracts session ID from signed cookie
func (s *StickySessionManager) extractSessionID(cookieValue string) string {
	parts := strings.Split(cookieValue, ".")
	if len(parts) != 2 {
		return ""
	}
	return parts[0]
}

// getClientIP gets the client IP address from request
func (s *StickySessionManager) getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		ips := strings.Split(forwarded, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// Fallback to RemoteAddr
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

// ConsistentHash provides consistent hashing for load balancing
type ConsistentHash struct {
	instances []string
}

// NewConsistentHash creates a new consistent hash
func NewConsistentHash(instances []string) *ConsistentHash {
	return &ConsistentHash{
		instances: instances,
	}
}

// GetInstance gets the instance for a given key using consistent hashing
func (ch *ConsistentHash) GetInstance(key string) string {
	if len(ch.instances) == 0 {
		return ""
	}

	hash := sha256.Sum256([]byte(key))
	hashValue := uint64(hash[0])<<56 | uint64(hash[1])<<48 | uint64(hash[2])<<40 | uint64(hash[3])<<32 |
		uint64(hash[4])<<24 | uint64(hash[5])<<16 | uint64(hash[6])<<8 | uint64(hash[7])

	index := int(hashValue % uint64(len(ch.instances)))
	return ch.instances[index]
}

