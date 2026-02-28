package web

import (
	"bytes"
	"image/png"
	"log"
	"net/http"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/qr"
	"github.com/mauv0809/ideal-tribble/internal/auth"
	"github.com/mauv0809/ideal-tribble/internal/web/templates"
)

// ProfileHandlers handles profile-related HTTP requests.
type ProfileHandlers struct {
	middleware     *Middleware
	authStore      *auth.Store
	sessionManager *auth.SessionManager
	totpManager    *auth.TOTPManager
}

// NewProfileHandlers creates a new profile handlers instance.
func NewProfileHandlers(
	middleware *Middleware,
	authStore *auth.Store,
	sessionManager *auth.SessionManager,
	totpManager *auth.TOTPManager,
) *ProfileHandlers {
	return &ProfileHandlers{
		middleware:     middleware,
		authStore:      authStore,
		sessionManager: sessionManager,
		totpManager:    totpManager,
	}
}

// ProfilePage renders the user profile page.
func (h *ProfileHandlers) ProfilePage(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)

	data := templates.PageData{
		Title: "Profile",
		User:  user,
	}

	if flashes := h.middleware.GetFlash(w, r, "success"); len(flashes) > 0 {
		data.FlashSuccess = flashes[0]
	}
	if flashes := h.middleware.GetFlash(w, r, "error"); len(flashes) > 0 {
		data.FlashError = flashes[0]
	}

	component := templates.ProfilePage(data)
	_ = component.Render(r.Context(), w)
}

// ChangePassword handles password change requests.
func (h *ProfileHandlers) ChangePassword(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)

	if err := r.ParseForm(); err != nil {
		_ = h.middleware.SetFlash(w, r, "error", "Invalid form data")
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	currentPassword := r.FormValue("current_password")
	newPassword := r.FormValue("new_password")
	confirmPassword := r.FormValue("confirm_password")

	// Validate current password
	fullUser, err := h.authStore.GetUserByID(user.ID)
	if err != nil || !h.authStore.ValidatePassword(fullUser, currentPassword) {
		_ = h.middleware.SetFlash(w, r, "error", "Current password is incorrect")
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	// Validate new password
	if len(newPassword) < 8 {
		_ = h.middleware.SetFlash(w, r, "error", "New password must be at least 8 characters")
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	if newPassword != confirmPassword {
		_ = h.middleware.SetFlash(w, r, "error", "New passwords do not match")
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	// Update password
	if err := h.authStore.UpdatePassword(user.ID, newPassword); err != nil {
		_ = h.middleware.SetFlash(w, r, "error", "Failed to update password")
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	_ = h.middleware.SetFlash(w, r, "success", "Password changed successfully")
	http.Redirect(w, r, "/profile", http.StatusSeeOther)
}

// TOTPSetupPage renders the TOTP setup page.
func (h *ProfileHandlers) TOTPSetupPage(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)
	log.Printf("[DEBUG TOTP] TOTPSetupPage called for user %s (ID: %d)", user.Email, user.ID)

	if user.TOTPEnabled {
		log.Printf("[DEBUG TOTP] User already has TOTP enabled, redirecting")
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	// Generate new TOTP secret
	encryptedSecret, provisioningURI, err := h.totpManager.GenerateSecret(user.Email)
	if err != nil {
		log.Printf("[DEBUG TOTP] Failed to generate secret: %v", err)
		_ = h.middleware.SetFlash(w, r, "error", "Failed to generate TOTP secret")
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}
	log.Printf("[DEBUG TOTP] Generated secret, encrypted length: %d", len(encryptedSecret))

	// Store the encrypted secret temporarily (we'll verify before enabling)
	if err := h.authStore.SetTOTPSecret(user.ID, encryptedSecret); err != nil {
		log.Printf("[DEBUG TOTP] Failed to save secret: %v", err)
		_ = h.middleware.SetFlash(w, r, "error", "Failed to save TOTP secret")
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}
	log.Printf("[DEBUG TOTP] Saved secret to database for user ID %d", user.ID)

	// Extract the raw secret from the provisioning URI for display
	// The secret is between "secret=" and "&" or end of string
	secret := extractSecretFromURI(provisioningURI)
	log.Printf("[DEBUG TOTP] Extracted secret from URI, length: %d", len(secret))

	data := templates.TOTPSetupData{
		PageData: templates.PageData{
			Title: "Setup 2FA",
			User:  user,
		},
		ProvisioningURI: provisioningURI,
		Secret:          secret,
	}

	component := templates.TOTPSetupPage(data)
	_ = component.Render(r.Context(), w)
}

// TOTPQRCode generates and serves the QR code image.
func (h *ProfileHandlers) TOTPQRCode(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)
	log.Printf("[DEBUG TOTP] TOTPQRCode called for user %s (ID: %d)", user.Email, user.ID)

	// Get the user's current TOTP secret
	fullUser, err := h.authStore.GetUserByID(user.ID)
	if err != nil {
		log.Printf("[DEBUG TOTP] Failed to get user: %v", err)
		http.Error(w, "No TOTP secret configured", http.StatusBadRequest)
		return
	}
	if fullUser.TOTPSecret == "" {
		log.Printf("[DEBUG TOTP] User has no TOTP secret stored")
		http.Error(w, "No TOTP secret configured", http.StatusBadRequest)
		return
	}
	log.Printf("[DEBUG TOTP] Found stored secret, length: %d", len(fullUser.TOTPSecret))

	// Get provisioning URI from the stored secret
	provisioningURI, err := h.totpManager.GetProvisioningURI(fullUser.TOTPSecret, user.Email)
	if err != nil {
		log.Printf("[DEBUG TOTP] Failed to get provisioning URI: %v", err)
		http.Error(w, "Failed to generate QR code", http.StatusInternalServerError)
		return
	}
	log.Printf("[DEBUG TOTP] Generated provisioning URI from stored secret")

	// Generate QR code
	qrCode, err := qr.Encode(provisioningURI, qr.M, qr.Auto)
	if err != nil {
		http.Error(w, "Failed to generate QR code", http.StatusInternalServerError)
		return
	}

	qrCode, err = barcode.Scale(qrCode, 200, 200)
	if err != nil {
		http.Error(w, "Failed to scale QR code", http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, qrCode); err != nil {
		http.Error(w, "Failed to encode QR code", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(buf.Bytes())
}

// VerifyTOTP verifies the TOTP code and enables 2FA.
func (h *ProfileHandlers) VerifyTOTP(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)
	log.Printf("[DEBUG TOTP] VerifyTOTP called for user %s (ID: %d)", user.Email, user.ID)

	if err := r.ParseForm(); err != nil {
		log.Printf("[DEBUG TOTP] Failed to parse form: %v", err)
		_ = h.middleware.SetFlash(w, r, "error", "Invalid form data")
		http.Redirect(w, r, "/profile/totp/setup", http.StatusSeeOther)
		return
	}

	code := r.FormValue("code")
	log.Printf("[DEBUG TOTP] Received verification code")

	// Get the stored secret
	fullUser, err := h.authStore.GetUserByID(user.ID)
	if err != nil {
		log.Printf("[DEBUG TOTP] Failed to get user: %v", err)
		_ = h.middleware.SetFlash(w, r, "error", "No TOTP secret found. Please start setup again.")
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}
	if fullUser.TOTPSecret == "" {
		log.Printf("[DEBUG TOTP] User has no TOTP secret stored")
		_ = h.middleware.SetFlash(w, r, "error", "No TOTP secret found. Please start setup again.")
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}
	log.Printf("[DEBUG TOTP] Found stored secret for validation, length: %d", len(fullUser.TOTPSecret))

	// Validate the code
	valid, err := h.totpManager.ValidateCode(fullUser.TOTPSecret, code)
	log.Printf("[DEBUG TOTP] Validation result: valid=%v, err=%v", valid, err)
	if err != nil || !valid {
		_ = h.middleware.SetFlash(w, r, "error", "Invalid verification code. Please try again.")
		http.Redirect(w, r, "/profile/totp/setup", http.StatusSeeOther)
		return
	}

	// Enable TOTP
	if err := h.authStore.EnableTOTP(user.ID); err != nil {
		log.Printf("[DEBUG TOTP] Failed to enable TOTP: %v", err)
		_ = h.middleware.SetFlash(w, r, "error", "Failed to enable 2FA")
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	log.Printf("[DEBUG TOTP] TOTP enabled successfully for user %d", user.ID)
	_ = h.middleware.SetFlash(w, r, "success", "Two-factor authentication enabled successfully")
	http.Redirect(w, r, "/profile", http.StatusSeeOther)
}

// DisableTOTP disables 2FA for the user.
func (h *ProfileHandlers) DisableTOTP(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)

	if err := h.authStore.DisableTOTP(user.ID); err != nil {
		_ = h.middleware.SetFlash(w, r, "error", "Failed to disable 2FA")
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	_ = h.middleware.SetFlash(w, r, "success", "Two-factor authentication disabled")
	http.Redirect(w, r, "/profile", http.StatusSeeOther)
}

// UpdateTheme handles theme preference changes via AJAX.
func (h *ProfileHandlers) UpdateTheme(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	theme := r.FormValue("theme")
	if err := h.authStore.SetTheme(user.ID, theme); err != nil {
		http.Error(w, "Failed to save theme", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func extractSecretFromURI(uri string) string {
	const prefix = "secret="
	start := 0
	for i := 0; i <= len(uri)-len(prefix); i++ {
		if uri[i:i+len(prefix)] == prefix {
			start = i + len(prefix)
			break
		}
	}
	if start == 0 {
		return ""
	}

	end := len(uri)
	for i := start; i < len(uri); i++ {
		if uri[i] == '&' {
			end = i
			break
		}
	}
	return uri[start:end]
}
