package utils

import (
	"fmt"
	"time"
)

// FormatDuration formats duration in human-readable format
func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
	if d < time.Hour {
		minutes := d / time.Minute
		seconds := (d % time.Minute) / time.Second
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	}
	hours := d / time.Hour
	minutes := (d % time.Hour) / time.Minute
	return fmt.Sprintf("%dh%dm", hours, minutes)
}

// ParseDurationSafe safely parses duration string
func ParseDurationSafe(s string, defaultDuration time.Duration) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return defaultDuration
	}
	return d
}

// IsExpired checks if a timestamp is expired
func IsExpired(timestamp time.Time, ttl time.Duration) bool {
	return time.Since(timestamp) > ttl
}

// TimeUntilExpiry returns time until expiry
func TimeUntilExpiry(timestamp time.Time, ttl time.Duration) time.Duration {
	expiryTime := timestamp.Add(ttl)
	remaining := time.Until(expiryTime)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// FormatTimestamp formats timestamp in ISO 8601 format
func FormatTimestamp(t time.Time) string {
	return t.Format(time.RFC3339)
}

// ParseTimestamp parses ISO 8601 timestamp
func ParseTimestamp(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}

// Now returns current time (useful for mocking in tests)
var Now = time.Now

// Since returns time since given time
func Since(t time.Time) time.Duration {
	return Now().Sub(t)
}

