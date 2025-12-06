package auth

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)

	// Create tables
	_, err = db.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			totp_secret TEXT,
			totp_enabled INTEGER NOT NULL DEFAULT 0,
			is_admin INTEGER NOT NULL DEFAULT 0,
			theme TEXT DEFAULT 'mocha',
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);

		CREATE TABLE sessions (
			id TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL,
			expires_at INTEGER NOT NULL,
			created_at INTEGER NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		);

		CREATE TABLE login_attempts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ip_address TEXT NOT NULL,
			email TEXT,
			attempted_at INTEGER NOT NULL,
			success INTEGER NOT NULL DEFAULT 0
		);
	`)
	require.NoError(t, err)

	return db
}

func TestCreateUser(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStore(db)

	t.Run("creates user successfully", func(t *testing.T) {
		user, err := store.CreateUser("test@example.com", "password123", false)
		require.NoError(t, err)
		assert.NotZero(t, user.ID)
		assert.Equal(t, "test@example.com", user.Email)
		assert.False(t, user.IsAdmin)
		assert.NotZero(t, user.CreatedAt)
	})

	t.Run("creates admin user", func(t *testing.T) {
		user, err := store.CreateUser("admin@example.com", "adminpass", true)
		require.NoError(t, err)
		assert.True(t, user.IsAdmin)
	})

	t.Run("fails on duplicate email", func(t *testing.T) {
		_, err := store.CreateUser("test@example.com", "anotherpass", false)
		assert.ErrorIs(t, err, ErrEmailAlreadyExists)
	})
}

func TestGetUserByEmail(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStore(db)

	t.Run("retrieves existing user", func(t *testing.T) {
		created, err := store.CreateUser("find@example.com", "password", false)
		require.NoError(t, err)

		found, err := store.GetUserByEmail("find@example.com")
		require.NoError(t, err)
		assert.Equal(t, created.ID, found.ID)
		assert.Equal(t, "find@example.com", found.Email)
	})

	t.Run("returns error for non-existent user", func(t *testing.T) {
		_, err := store.GetUserByEmail("nonexistent@example.com")
		assert.ErrorIs(t, err, ErrUserNotFound)
	})
}

func TestValidatePassword(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStore(db)

	user, err := store.CreateUser("validate@example.com", "correctpassword", false)
	require.NoError(t, err)

	// Need to fetch user to get password hash
	fetchedUser, err := store.GetUserByEmail("validate@example.com")
	require.NoError(t, err)

	t.Run("validates correct password", func(t *testing.T) {
		assert.True(t, store.ValidatePassword(fetchedUser, "correctpassword"))
	})

	t.Run("rejects incorrect password", func(t *testing.T) {
		assert.False(t, store.ValidatePassword(fetchedUser, "wrongpassword"))
	})

	_ = user // silence unused warning
}

func TestUpdatePassword(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStore(db)

	user, err := store.CreateUser("update@example.com", "oldpassword", false)
	require.NoError(t, err)

	t.Run("updates password successfully", func(t *testing.T) {
		err := store.UpdatePassword(user.ID, "newpassword")
		require.NoError(t, err)

		fetchedUser, err := store.GetUserByEmail("update@example.com")
		require.NoError(t, err)

		assert.False(t, store.ValidatePassword(fetchedUser, "oldpassword"))
		assert.True(t, store.ValidatePassword(fetchedUser, "newpassword"))
	})

	t.Run("fails for non-existent user", func(t *testing.T) {
		err := store.UpdatePassword(9999, "password")
		assert.ErrorIs(t, err, ErrUserNotFound)
	})
}

func TestTOTPOperations(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStore(db)

	user, err := store.CreateUser("totp@example.com", "password", false)
	require.NoError(t, err)

	t.Run("sets TOTP secret", func(t *testing.T) {
		err := store.SetTOTPSecret(user.ID, "encrypted_secret_here")
		require.NoError(t, err)

		fetchedUser, err := store.GetUserByID(user.ID)
		require.NoError(t, err)
		assert.Equal(t, "encrypted_secret_here", fetchedUser.TOTPSecret)
	})

	t.Run("enables TOTP", func(t *testing.T) {
		err := store.EnableTOTP(user.ID)
		require.NoError(t, err)

		fetchedUser, err := store.GetUserByID(user.ID)
		require.NoError(t, err)
		assert.True(t, fetchedUser.TOTPEnabled)
	})

	t.Run("disables TOTP", func(t *testing.T) {
		err := store.DisableTOTP(user.ID)
		require.NoError(t, err)

		fetchedUser, err := store.GetUserByID(user.ID)
		require.NoError(t, err)
		assert.False(t, fetchedUser.TOTPEnabled)
		assert.Empty(t, fetchedUser.TOTPSecret)
	})
}

func TestListUsers(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStore(db)

	_, err := store.CreateUser("user1@example.com", "pass", false)
	require.NoError(t, err)
	_, err = store.CreateUser("user2@example.com", "pass", true)
	require.NoError(t, err)

	users, err := store.ListUsers()
	require.NoError(t, err)
	assert.Len(t, users, 2)
}

func TestDeleteUser(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStore(db)

	user, err := store.CreateUser("delete@example.com", "pass", false)
	require.NoError(t, err)

	t.Run("deletes existing user", func(t *testing.T) {
		err := store.DeleteUser(user.ID)
		require.NoError(t, err)

		_, err = store.GetUserByID(user.ID)
		assert.ErrorIs(t, err, ErrUserNotFound)
	})

	t.Run("fails for non-existent user", func(t *testing.T) {
		err := store.DeleteUser(9999)
		assert.ErrorIs(t, err, ErrUserNotFound)
	})
}

func TestLoginAttempts(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStore(db)

	t.Run("records login attempts", func(t *testing.T) {
		err := store.RecordLoginAttempt("192.168.1.1", "test@example.com", false)
		require.NoError(t, err)

		err = store.RecordLoginAttempt("192.168.1.1", "test@example.com", false)
		require.NoError(t, err)

		err = store.RecordLoginAttempt("192.168.1.1", "test@example.com", true)
		require.NoError(t, err)
	})

	t.Run("counts recent failed attempts", func(t *testing.T) {
		count, err := store.GetRecentLoginAttempts("192.168.1.1", time.Hour)
		require.NoError(t, err)
		assert.Equal(t, 2, count) // Only failed attempts
	})
}

func TestLoginAttemptsCleanup(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStore(db)

	// Record some attempts
	err := store.RecordLoginAttempt("192.168.1.2", "cleanup@example.com", false)
	require.NoError(t, err)

	// Cleanup with a duration in the future (removes all attempts older than now+1hr = none)
	err = store.CleanupOldLoginAttempts(-time.Hour)
	require.NoError(t, err)

	// All attempts should be removed (because -1hr means cutoff is 1hr in the future)
	count, err := store.GetRecentLoginAttempts("192.168.1.2", time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestUserCount(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewStore(db)

	count, err := store.UserCount()
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	_, err = store.CreateUser("count@example.com", "pass", false)
	require.NoError(t, err)

	count, err = store.UserCount()
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}
