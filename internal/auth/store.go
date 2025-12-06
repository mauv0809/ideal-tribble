package auth

import (
	"database/sql"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	// BcryptCost is the cost factor for bcrypt password hashing.
	BcryptCost = 12
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
)

// Store handles user persistence operations.
type Store struct {
	db *sql.DB
}

// NewStore creates a new auth store.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// CreateUser creates a new user with a hashed password.
func (s *Store) CreateUser(email, password string, isAdmin bool) (*User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return nil, err
	}

	now := time.Now().Unix()

	result, err := s.db.Exec(`
		INSERT INTO users (email, password_hash, is_admin, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`, email, string(hash), boolToInt(isAdmin), now, now)
	if err != nil {
		if isUniqueConstraintError(err) {
			return nil, ErrEmailAlreadyExists
		}
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &User{
		ID:        id,
		Email:     email,
		IsAdmin:   isAdmin,
		CreatedAt: time.Unix(now, 0),
		UpdatedAt: time.Unix(now, 0),
	}, nil
}

// GetUserByID retrieves a user by ID.
func (s *Store) GetUserByID(id int64) (*User, error) {
	var user User
	var createdAt, updatedAt int64
	var totpEnabled, isAdmin int
	var totpSecret sql.NullString

	err := s.db.QueryRow(`
		SELECT id, email, password_hash, totp_secret, totp_enabled, is_admin, created_at, updated_at
		FROM users WHERE id = ?
	`, id).Scan(
		&user.ID, &user.Email, &user.PasswordHash,
		&totpSecret, &totpEnabled, &isAdmin,
		&createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	user.TOTPSecret = totpSecret.String
	user.TOTPEnabled = totpEnabled == 1
	user.IsAdmin = isAdmin == 1
	user.CreatedAt = time.Unix(createdAt, 0)
	user.UpdatedAt = time.Unix(updatedAt, 0)

	return &user, nil
}

// GetUserByEmail retrieves a user by email.
func (s *Store) GetUserByEmail(email string) (*User, error) {
	var user User
	var createdAt, updatedAt int64
	var totpEnabled, isAdmin int
	var totpSecret sql.NullString

	err := s.db.QueryRow(`
		SELECT id, email, password_hash, totp_secret, totp_enabled, is_admin, created_at, updated_at
		FROM users WHERE email = ?
	`, email).Scan(
		&user.ID, &user.Email, &user.PasswordHash,
		&totpSecret, &totpEnabled, &isAdmin,
		&createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	user.TOTPSecret = totpSecret.String
	user.TOTPEnabled = totpEnabled == 1
	user.IsAdmin = isAdmin == 1
	user.CreatedAt = time.Unix(createdAt, 0)
	user.UpdatedAt = time.Unix(updatedAt, 0)

	return &user, nil
}

// ValidatePassword checks if the provided password matches the user's hash.
func (s *Store) ValidatePassword(user *User, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	return err == nil
}

// UpdatePassword updates a user's password.
func (s *Store) UpdatePassword(userID int64, newPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), BcryptCost)
	if err != nil {
		return err
	}

	now := time.Now().Unix()

	result, err := s.db.Exec(`
		UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?
	`, string(hash), now, userID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrUserNotFound
	}

	return nil
}

// SetTOTPSecret sets the encrypted TOTP secret for a user.
func (s *Store) SetTOTPSecret(userID int64, encryptedSecret string) error {
	now := time.Now().Unix()

	result, err := s.db.Exec(`
		UPDATE users SET totp_secret = ?, updated_at = ? WHERE id = ?
	`, encryptedSecret, now, userID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrUserNotFound
	}

	return nil
}

// EnableTOTP enables TOTP for a user.
func (s *Store) EnableTOTP(userID int64) error {
	now := time.Now().Unix()

	result, err := s.db.Exec(`
		UPDATE users SET totp_enabled = 1, updated_at = ? WHERE id = ?
	`, now, userID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrUserNotFound
	}

	return nil
}

// DisableTOTP disables TOTP for a user.
func (s *Store) DisableTOTP(userID int64) error {
	now := time.Now().Unix()

	result, err := s.db.Exec(`
		UPDATE users SET totp_enabled = 0, totp_secret = NULL, updated_at = ? WHERE id = ?
	`, now, userID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrUserNotFound
	}

	return nil
}

// ListUsers returns all users.
func (s *Store) ListUsers() ([]*User, error) {
	rows, err := s.db.Query(`
		SELECT id, email, totp_enabled, is_admin, created_at, updated_at
		FROM users ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		var user User
		var createdAt, updatedAt int64
		var totpEnabled, isAdmin int

		err := rows.Scan(&user.ID, &user.Email, &totpEnabled, &isAdmin, &createdAt, &updatedAt)
		if err != nil {
			return nil, err
		}

		user.TOTPEnabled = totpEnabled == 1
		user.IsAdmin = isAdmin == 1
		user.CreatedAt = time.Unix(createdAt, 0)
		user.UpdatedAt = time.Unix(updatedAt, 0)

		users = append(users, &user)
	}

	return users, rows.Err()
}

// DeleteUser deletes a user by ID.
func (s *Store) DeleteUser(id int64) error {
	result, err := s.db.Exec(`DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrUserNotFound
	}

	return nil
}

// UserCount returns the total number of users.
func (s *Store) UserCount() (int64, error) {
	var count int64
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	return count, err
}

// RecordLoginAttempt records a login attempt for rate limiting.
func (s *Store) RecordLoginAttempt(ipAddress, email string, success bool) error {
	now := time.Now().Unix()
	_, err := s.db.Exec(`
		INSERT INTO login_attempts (ip_address, email, attempted_at, success)
		VALUES (?, ?, ?, ?)
	`, ipAddress, email, now, boolToInt(success))
	return err
}

// GetRecentLoginAttempts returns the number of failed login attempts in the given window.
func (s *Store) GetRecentLoginAttempts(ipAddress string, window time.Duration) (int, error) {
	since := time.Now().Add(-window).Unix()
	var count int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM login_attempts
		WHERE ip_address = ? AND attempted_at > ? AND success = 0
	`, ipAddress, since).Scan(&count)
	return count, err
}

// CleanupOldLoginAttempts removes login attempts older than the given duration.
func (s *Store) CleanupOldLoginAttempts(olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan).Unix()
	_, err := s.db.Exec(`DELETE FROM login_attempts WHERE attempted_at < ?`, cutoff)
	return err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func isUniqueConstraintError(err error) bool {
	return err != nil && (contains(err.Error(), "UNIQUE constraint failed") ||
		contains(err.Error(), "duplicate key"))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr, 0))
}

func containsAt(s, substr string, start int) bool {
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
