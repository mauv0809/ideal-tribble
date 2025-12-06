package auth

import (
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateSession(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create a user first
	store := NewStore(db)
	user, err := store.CreateUser("session@example.com", "password", false)
	require.NoError(t, err)

	manager := NewSessionManager(db)

	t.Run("creates session successfully", func(t *testing.T) {
		session, err := manager.CreateSession(user.ID)
		require.NoError(t, err)
		assert.NotEmpty(t, session.ID)
		assert.Equal(t, user.ID, session.UserID)
		assert.True(t, session.ExpiresAt.After(time.Now()))
	})
}

func TestGetSession(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStore(db)
	user, err := store.CreateUser("get@example.com", "password", false)
	require.NoError(t, err)

	manager := NewSessionManager(db)
	created, err := manager.CreateSession(user.ID)
	require.NoError(t, err)

	t.Run("retrieves existing session", func(t *testing.T) {
		session, err := manager.GetSession(created.ID)
		require.NoError(t, err)
		assert.Equal(t, created.ID, session.ID)
		assert.Equal(t, user.ID, session.UserID)
	})

	t.Run("returns error for non-existent session", func(t *testing.T) {
		_, err := manager.GetSession("nonexistent_token")
		assert.ErrorIs(t, err, ErrSessionNotFound)
	})
}

func TestValidateSession(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStore(db)
	user, err := store.CreateUser("validate@example.com", "password", false)
	require.NoError(t, err)

	manager := NewSessionManager(db)
	session, err := manager.CreateSession(user.ID)
	require.NoError(t, err)

	t.Run("validates existing session", func(t *testing.T) {
		userID, err := manager.ValidateSession(session.ID)
		require.NoError(t, err)
		assert.Equal(t, user.ID, userID)
	})

	t.Run("returns error for invalid token", func(t *testing.T) {
		_, err := manager.ValidateSession("invalid_token")
		assert.ErrorIs(t, err, ErrSessionNotFound)
	})
}

func TestExpiredSession(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStore(db)
	user, err := store.CreateUser("expired@example.com", "password", false)
	require.NoError(t, err)

	manager := NewSessionManager(db)
	manager.SetSessionDuration(1 * time.Millisecond) // Very short expiry

	session, err := manager.CreateSession(user.ID)
	require.NoError(t, err)

	// Wait for session to expire
	time.Sleep(10 * time.Millisecond)

	t.Run("returns error for expired session", func(t *testing.T) {
		_, err := manager.GetSession(session.ID)
		assert.ErrorIs(t, err, ErrSessionExpired)
	})
}

func TestExtendSession(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStore(db)
	user, err := store.CreateUser("extend@example.com", "password", false)
	require.NoError(t, err)

	manager := NewSessionManager(db)
	manager.SetSessionDuration(1 * time.Hour)

	session, err := manager.CreateSession(user.ID)
	require.NoError(t, err)

	originalExpiry := session.ExpiresAt

	// Change duration and extend
	manager.SetSessionDuration(2 * time.Hour)
	err = manager.ExtendSession(session.ID)
	require.NoError(t, err)

	// Verify extension
	updated, err := manager.GetSession(session.ID)
	require.NoError(t, err)
	assert.True(t, updated.ExpiresAt.After(originalExpiry))
}

func TestDeleteSession(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStore(db)
	user, err := store.CreateUser("delete@example.com", "password", false)
	require.NoError(t, err)

	manager := NewSessionManager(db)
	session, err := manager.CreateSession(user.ID)
	require.NoError(t, err)

	t.Run("deletes session successfully", func(t *testing.T) {
		err := manager.DeleteSession(session.ID)
		require.NoError(t, err)

		_, err = manager.GetSession(session.ID)
		assert.ErrorIs(t, err, ErrSessionNotFound)
	})
}

func TestDeleteUserSessions(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStore(db)
	user, err := store.CreateUser("multiplelogout@example.com", "password", false)
	require.NoError(t, err)

	manager := NewSessionManager(db)

	// Create multiple sessions
	session1, err := manager.CreateSession(user.ID)
	require.NoError(t, err)
	session2, err := manager.CreateSession(user.ID)
	require.NoError(t, err)

	// Delete all user sessions
	err = manager.DeleteUserSessions(user.ID)
	require.NoError(t, err)

	// Both sessions should be gone
	_, err = manager.GetSession(session1.ID)
	assert.ErrorIs(t, err, ErrSessionNotFound)
	_, err = manager.GetSession(session2.ID)
	assert.ErrorIs(t, err, ErrSessionNotFound)
}

func TestCleanupExpiredSessions(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStore(db)
	user, err := store.CreateUser("cleanup@example.com", "password", false)
	require.NoError(t, err)

	manager := NewSessionManager(db)
	manager.SetSessionDuration(1 * time.Millisecond)

	// Create expired session
	expiredSession, err := manager.CreateSession(user.ID)
	require.NoError(t, err)

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	// Create a new valid session
	manager.SetSessionDuration(1 * time.Hour)
	validSession, err := manager.CreateSession(user.ID)
	require.NoError(t, err)

	// Cleanup
	err = manager.CleanupExpiredSessions()
	require.NoError(t, err)

	// Expired session should be gone (either not found or expired)
	_, err = manager.GetSession(expiredSession.ID)
	assert.True(t, err == ErrSessionNotFound || err == ErrSessionExpired)

	// Valid session should still exist
	_, err = manager.GetSession(validSession.ID)
	require.NoError(t, err)
}

func TestGenerateSecureToken(t *testing.T) {
	t.Run("generates unique tokens", func(t *testing.T) {
		token1, err := generateSecureToken(32)
		require.NoError(t, err)

		token2, err := generateSecureToken(32)
		require.NoError(t, err)

		assert.NotEqual(t, token1, token2)
		assert.NotEmpty(t, token1)
		assert.NotEmpty(t, token2)
	})

	t.Run("generates tokens of expected length", func(t *testing.T) {
		token, err := generateSecureToken(32)
		require.NoError(t, err)

		// Base64 encoding of 32 bytes = 44 characters (with padding)
		assert.Len(t, token, 44)
	})
}
