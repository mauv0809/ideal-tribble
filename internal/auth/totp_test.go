package auth

import (
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTOTPManager(t *testing.T) {
	t.Run("creates manager with valid key", func(t *testing.T) {
		key := make([]byte, 32)
		for i := range key {
			key[i] = byte(i)
		}

		manager, err := NewTOTPManager(key)
		require.NoError(t, err)
		assert.NotNil(t, manager)
	})

	t.Run("fails with empty key", func(t *testing.T) {
		_, err := NewTOTPManager([]byte{})
		assert.ErrorIs(t, err, ErrEncryptionKeyEmpty)
	})

	t.Run("fails with wrong key length", func(t *testing.T) {
		_, err := NewTOTPManager([]byte("short"))
		assert.ErrorIs(t, err, ErrInvalidKeyLength)

		_, err = NewTOTPManager(make([]byte, 16)) // AES-128 size
		assert.ErrorIs(t, err, ErrInvalidKeyLength)
	})
}

func TestGenerateSecret(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	manager, err := NewTOTPManager(key)
	require.NoError(t, err)

	t.Run("generates valid secret", func(t *testing.T) {
		encrypted, uri, err := manager.GenerateSecret("user@example.com")
		require.NoError(t, err)
		assert.NotEmpty(t, encrypted)
		assert.NotEmpty(t, uri)
		assert.Contains(t, uri, "otpauth://totp/")
		assert.Contains(t, uri, "user@example.com")
		assert.Contains(t, uri, TOTPIssuer)
	})

	t.Run("generates unique secrets", func(t *testing.T) {
		encrypted1, _, err := manager.GenerateSecret("user1@example.com")
		require.NoError(t, err)

		encrypted2, _, err := manager.GenerateSecret("user2@example.com")
		require.NoError(t, err)

		assert.NotEqual(t, encrypted1, encrypted2)
	})
}

func TestValidateCode(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	manager, err := NewTOTPManager(key)
	require.NoError(t, err)

	t.Run("validates correct code", func(t *testing.T) {
		// Generate a secret and get the raw secret from the URI
		encrypted, uri, err := manager.GenerateSecret("validate@example.com")
		require.NoError(t, err)

		// Decrypt to get raw secret for code generation
		rawSecret, err := manager.decrypt(encrypted)
		require.NoError(t, err)

		// Generate a valid code
		code, err := totp.GenerateCode(rawSecret, time.Now())
		require.NoError(t, err)

		// Validate
		valid, err := manager.ValidateCode(encrypted, code)
		require.NoError(t, err)
		assert.True(t, valid)

		_ = uri // silence unused warning
	})

	t.Run("rejects invalid code", func(t *testing.T) {
		encrypted, _, err := manager.GenerateSecret("invalid@example.com")
		require.NoError(t, err)

		valid, err := manager.ValidateCode(encrypted, "000000")
		require.NoError(t, err)
		assert.False(t, valid)
	})

	t.Run("returns error for empty secret", func(t *testing.T) {
		_, err := manager.ValidateCode("", "123456")
		assert.ErrorIs(t, err, ErrTOTPNotConfigured)
	})
}

func TestEncryptDecrypt(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	manager, err := NewTOTPManager(key)
	require.NoError(t, err)

	t.Run("encrypts and decrypts successfully", func(t *testing.T) {
		plaintext := "JBSWY3DPEHPK3PXP" // Example TOTP secret

		encrypted, err := manager.encrypt(plaintext)
		require.NoError(t, err)
		assert.NotEqual(t, plaintext, encrypted)

		decrypted, err := manager.decrypt(encrypted)
		require.NoError(t, err)
		assert.Equal(t, plaintext, decrypted)
	})

	t.Run("produces different ciphertext each time", func(t *testing.T) {
		plaintext := "SECRET"

		encrypted1, err := manager.encrypt(plaintext)
		require.NoError(t, err)

		encrypted2, err := manager.encrypt(plaintext)
		require.NoError(t, err)

		// Due to random nonce, ciphertexts should differ
		assert.NotEqual(t, encrypted1, encrypted2)

		// But both should decrypt to same value
		decrypted1, _ := manager.decrypt(encrypted1)
		decrypted2, _ := manager.decrypt(encrypted2)
		assert.Equal(t, decrypted1, decrypted2)
	})

	t.Run("fails to decrypt with wrong key", func(t *testing.T) {
		plaintext := "SECRET"
		encrypted, err := manager.encrypt(plaintext)
		require.NoError(t, err)

		// Create manager with different key
		differentKey := make([]byte, 32)
		for i := range differentKey {
			differentKey[i] = byte(i + 1)
		}
		differentManager, err := NewTOTPManager(differentKey)
		require.NoError(t, err)

		_, err = differentManager.decrypt(encrypted)
		assert.Error(t, err)
	})
}
