package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"log"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

const (
	// TOTPIssuer is the issuer name shown in authenticator apps.
	TOTPIssuer = "IdealTribble"
)

var (
	ErrInvalidTOTPCode    = errors.New("invalid TOTP code")
	ErrTOTPNotConfigured  = errors.New("TOTP not configured for user")
	ErrEncryptionKeyEmpty = errors.New("encryption key is empty")
	ErrInvalidKeyLength   = errors.New("encryption key must be 32 bytes")
)

// TOTPManager handles TOTP operations.
type TOTPManager struct {
	encryptionKey []byte // 32 bytes for AES-256
}

// NewTOTPManager creates a new TOTP manager with the given encryption key.
// The key must be 32 bytes for AES-256-GCM.
func NewTOTPManager(encryptionKey []byte) (*TOTPManager, error) {
	if len(encryptionKey) == 0 {
		return nil, ErrEncryptionKeyEmpty
	}
	if len(encryptionKey) != 32 {
		return nil, ErrInvalidKeyLength
	}
	return &TOTPManager{encryptionKey: encryptionKey}, nil
}

// GenerateSecret generates a new TOTP secret for a user.
// Returns the encrypted secret (for storage) and the provisioning URI (for QR code).
func (m *TOTPManager) GenerateSecret(userEmail string) (encryptedSecret string, provisioningURI string, err error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      TOTPIssuer,
		AccountName: userEmail,
		Algorithm:   otp.AlgorithmSHA1,
		Digits:      otp.DigitsSix,
	})
	if err != nil {
		return "", "", err
	}

	log.Printf("[DEBUG TOTP] Generated new TOTP secret")

	// Encrypt the secret for storage
	encrypted, err := m.encrypt(key.Secret())
	if err != nil {
		return "", "", err
	}

	log.Printf("[DEBUG TOTP] Encrypted secret length: %d", len(encrypted))
	return encrypted, key.URL(), nil
}

// GetProvisioningURI generates a provisioning URI from an existing encrypted secret.
func (m *TOTPManager) GetProvisioningURI(encryptedSecret, userEmail string) (string, error) {
	log.Printf("[DEBUG TOTP] GetProvisioningURI called, encrypted length: %d", len(encryptedSecret))
	secret, err := m.decrypt(encryptedSecret)
	if err != nil {
		log.Printf("[DEBUG TOTP] Failed to decrypt: %v", err)
		return "", err
	}

	log.Printf("[DEBUG TOTP] Decrypted secret successfully")

	// Build the otpauth URI manually
	// Format: otpauth://totp/ISSUER:ACCOUNT?secret=SECRET&issuer=ISSUER&algorithm=SHA1&digits=6
	uri := "otpauth://totp/" + TOTPIssuer + ":" + userEmail + "?secret=" + secret + "&issuer=" + TOTPIssuer + "&algorithm=SHA1&digits=6"
	log.Printf("[DEBUG TOTP] Generated provisioning URI")
	return uri, nil
}

// ValidateCode validates a TOTP code against an encrypted secret.
func (m *TOTPManager) ValidateCode(encryptedSecret, code string) (bool, error) {
	log.Printf("[DEBUG TOTP] ValidateCode called, encrypted length: %d", len(encryptedSecret))
	if encryptedSecret == "" {
		return false, ErrTOTPNotConfigured
	}

	secret, err := m.decrypt(encryptedSecret)
	if err != nil {
		log.Printf("[DEBUG TOTP] Failed to decrypt for validation: %v", err)
		return false, err
	}

	log.Printf("[DEBUG TOTP] Decrypted secret for validation")
	valid := totp.Validate(code, secret)
	log.Printf("[DEBUG TOTP] Validation result: %v", valid)
	return valid, nil
}

// encrypt encrypts plaintext using AES-256-GCM.
func (m *TOTPManager) encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(m.encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decrypt decrypts ciphertext encrypted with AES-256-GCM.
func (m *TOTPManager) decrypt(encrypted string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(m.encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	if len(ciphertext) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
