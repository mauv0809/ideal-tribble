package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"time"
)

const (
	// SessionTokenLength is the length of session tokens in bytes (before base64 encoding).
	SessionTokenLength = 32

	// DefaultSessionDuration is the default session lifetime.
	DefaultSessionDuration = 24 * time.Hour
)

var (
	ErrSessionNotFound = errors.New("session not found")
	ErrSessionExpired  = errors.New("session expired")
)

// SessionManager handles session operations.
type SessionManager struct {
	db              *sql.DB
	sessionDuration time.Duration
}

// NewSessionManager creates a new session manager.
func NewSessionManager(db *sql.DB) *SessionManager {
	return &SessionManager{
		db:              db,
		sessionDuration: DefaultSessionDuration,
	}
}

// SetSessionDuration configures the session lifetime.
func (m *SessionManager) SetSessionDuration(d time.Duration) {
	m.sessionDuration = d
}

// CreateSession creates a new session for a user.
func (m *SessionManager) CreateSession(userID int64) (*Session, error) {
	token, err := generateSecureToken(SessionTokenLength)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	expiresAt := now.Add(m.sessionDuration)

	_, err = m.db.Exec(`
		INSERT INTO sessions (id, user_id, expires_at, created_at)
		VALUES (?, ?, ?, ?)
	`, token, userID, expiresAt.Unix(), now.Unix())
	if err != nil {
		return nil, err
	}

	return &Session{
		ID:        token,
		UserID:    userID,
		ExpiresAt: expiresAt,
		CreatedAt: now,
	}, nil
}

// GetSession retrieves a session by its token.
func (m *SessionManager) GetSession(token string) (*Session, error) {
	var session Session
	var expiresAt, createdAt int64

	err := m.db.QueryRow(`
		SELECT id, user_id, expires_at, created_at
		FROM sessions WHERE id = ?
	`, token).Scan(&session.ID, &session.UserID, &expiresAt, &createdAt)
	if err == sql.ErrNoRows {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, err
	}

	session.ExpiresAt = time.Unix(expiresAt, 0)
	session.CreatedAt = time.Unix(createdAt, 0)

	// Check expiration
	if time.Now().After(session.ExpiresAt) {
		// Clean up expired session
		_ = m.DeleteSession(token)
		return nil, ErrSessionExpired
	}

	return &session, nil
}

// ValidateSession checks if a session token is valid and not expired.
// Returns the user ID if valid.
func (m *SessionManager) ValidateSession(token string) (int64, error) {
	session, err := m.GetSession(token)
	if err != nil {
		return 0, err
	}
	return session.UserID, nil
}

// ExtendSession extends the session expiration (sliding window).
func (m *SessionManager) ExtendSession(token string) error {
	newExpiry := time.Now().Add(m.sessionDuration).Unix()

	result, err := m.db.Exec(`
		UPDATE sessions SET expires_at = ? WHERE id = ?
	`, newExpiry, token)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrSessionNotFound
	}

	return nil
}

// DeleteSession removes a session (logout).
func (m *SessionManager) DeleteSession(token string) error {
	_, err := m.db.Exec(`DELETE FROM sessions WHERE id = ?`, token)
	return err
}

// DeleteUserSessions removes all sessions for a user.
func (m *SessionManager) DeleteUserSessions(userID int64) error {
	_, err := m.db.Exec(`DELETE FROM sessions WHERE user_id = ?`, userID)
	return err
}

// CleanupExpiredSessions removes all expired sessions.
func (m *SessionManager) CleanupExpiredSessions() error {
	now := time.Now().Unix()
	_, err := m.db.Exec(`DELETE FROM sessions WHERE expires_at < ?`, now)
	return err
}

// generateSecureToken generates a cryptographically secure random token.
func generateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}
