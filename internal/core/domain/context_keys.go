package domain

// contextKey is an unexported type for context keys to avoid collisions (SA1029).
type contextKey string

// UserIDContextKey carries the authenticated user ID in request context.
const UserIDContextKey contextKey = "user_id"
