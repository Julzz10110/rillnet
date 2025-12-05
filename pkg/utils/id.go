package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// GenerateStreamID generates a unique stream ID
func GenerateStreamID() string {
	return GenerateID("stream")
}

// GeneratePeerID generates a unique peer ID
func GeneratePeerID() string {
	return GenerateID("peer")
}

// GenerateSessionID generates a unique session ID
func GenerateSessionID() string {
	return GenerateID("session")
}

// GenerateUserID generates a unique user ID
func GenerateUserID() string {
	return GenerateID("user")
}

// GenerateRequestID generates a unique request ID
func GenerateRequestID() string {
	timestamp := time.Now().UnixNano()
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("req_%d_%s", timestamp, hex.EncodeToString(b))
}

// GenerateTraceID generates a unique trace ID
func GenerateTraceID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

