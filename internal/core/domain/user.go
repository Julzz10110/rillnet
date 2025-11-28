package domain

import "time"

type UserID string

type User struct {
	ID        UserID
	Username  string
	Email     string
	CreatedAt time.Time
}

type UserRole string

const (
	RoleOwner     UserRole = "owner"
	RoleViewer    UserRole = "viewer"
	RoleModerator UserRole = "moderator"
)

type StreamPermission struct {
	StreamID  StreamID
	UserID    UserID
	Role      UserRole
	GrantedAt time.Time
}
