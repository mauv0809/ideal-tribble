package auth

import "time"

// User represents an authenticated user in the system.
type User struct {
	ID           int64
	Email        string
	PasswordHash string
	TOTPSecret   string // AES-GCM encrypted
	TOTPEnabled  bool
	IsAdmin      bool
	Theme        string // catppuccin theme: mocha, latte, frappe, macchiato
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Session represents an active user session.
type Session struct {
	ID        string // Secure random token
	UserID    int64
	ExpiresAt time.Time
	CreatedAt time.Time
}

// LoginAttempt tracks login attempts for rate limiting.
type LoginAttempt struct {
	ID          int64
	IPAddress   string
	Email       string
	AttemptedAt time.Time
	Success     bool
}
