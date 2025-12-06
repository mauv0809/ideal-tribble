package web

import (
	"net/http"

	"github.com/mauv0809/ideal-tribble/internal/auth"
	"github.com/mauv0809/ideal-tribble/internal/web/templates"
)

// AuthHandlers handles authentication-related HTTP requests.
type AuthHandlers struct {
	middleware     *Middleware
	authStore      *auth.Store
	sessionManager *auth.SessionManager
	totpManager    *auth.TOTPManager
	rateLimiter    *auth.IPRateLimiter
}

// NewAuthHandlers creates a new auth handlers instance.
func NewAuthHandlers(
	middleware *Middleware,
	authStore *auth.Store,
	sessionManager *auth.SessionManager,
	totpManager *auth.TOTPManager,
	rateLimiter *auth.IPRateLimiter,
) *AuthHandlers {
	return &AuthHandlers{
		middleware:     middleware,
		authStore:      authStore,
		sessionManager: sessionManager,
		totpManager:    totpManager,
		rateLimiter:    rateLimiter,
	}
}

// LoginPage renders the login form.
func (h *AuthHandlers) LoginPage(w http.ResponseWriter, r *http.Request) {
	// If already authenticated, redirect to dashboard
	user := h.middleware.getUserFromSession(r)
	if user != nil {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}

	data := templates.PageData{
		Title: "Login",
	}

	// Check for flash messages
	if flashes := h.middleware.GetFlash(w, r, "error"); len(flashes) > 0 {
		data.FlashError = flashes[0]
	}

	component := templates.LoginPage(data, false)
	_ = component.Render(r.Context(), w)
}

// HandleLogin processes the login form.
func (h *AuthHandlers) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.loginError(w, r, "Invalid form data")
		return
	}

	email := r.FormValue("email")
	password := r.FormValue("password")
	totpCode := r.FormValue("totp_code")

	ip := getClientIP(r)

	// Rate limiting check
	if !h.rateLimiter.AllowIP(ip) {
		h.loginError(w, r, "Too many login attempts. Please try again later.")
		return
	}

	// Find user
	user, err := h.authStore.GetUserByEmail(email)
	if err != nil {
		_ = h.authStore.RecordLoginAttempt(ip, email, false)
		h.loginError(w, r, "Invalid email or password")
		return
	}

	// Validate password
	if !h.authStore.ValidatePassword(user, password) {
		_ = h.authStore.RecordLoginAttempt(ip, email, false)
		h.loginError(w, r, "Invalid email or password")
		return
	}

	// Check TOTP if enabled
	if user.TOTPEnabled {
		if totpCode == "" {
			// Show TOTP entry page
			data := templates.PageData{Title: "Two-Factor Authentication"}
			component := templates.TOTPRequiredPage(data, email)
			_ = component.Render(r.Context(), w)
			return
		}

		valid, err := h.totpManager.ValidateCode(user.TOTPSecret, totpCode)
		if err != nil || !valid {
			_ = h.authStore.RecordLoginAttempt(ip, email, false)
			h.loginError(w, r, "Invalid two-factor code")
			return
		}
	}

	// Create session
	session, err := h.sessionManager.CreateSession(user.ID)
	if err != nil {
		h.loginError(w, r, "Failed to create session")
		return
	}

	// Store session token in cookie
	if err := h.middleware.SetSessionToken(w, r, session.ID); err != nil {
		h.loginError(w, r, "Failed to save session")
		return
	}

	// Record successful login
	_ = h.authStore.RecordLoginAttempt(ip, email, true)
	h.rateLimiter.ResetIP(ip)

	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

// HandleTOTPLogin handles the TOTP verification after password login.
func (h *AuthHandlers) HandleTOTPLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.loginError(w, r, "Invalid form data")
		return
	}

	email := r.FormValue("email")
	totpCode := r.FormValue("totp_code")
	ip := getClientIP(r)

	// Rate limiting check
	if !h.rateLimiter.AllowIP(ip) {
		h.loginError(w, r, "Too many login attempts. Please try again later.")
		return
	}

	// Find user
	user, err := h.authStore.GetUserByEmail(email)
	if err != nil {
		h.loginError(w, r, "Invalid credentials")
		return
	}

	// Validate TOTP
	valid, err := h.totpManager.ValidateCode(user.TOTPSecret, totpCode)
	if err != nil || !valid {
		_ = h.authStore.RecordLoginAttempt(ip, email, false)
		data := templates.PageData{
			Title:      "Two-Factor Authentication",
			FlashError: "Invalid two-factor code",
		}
		component := templates.TOTPRequiredPage(data, email)
		_ = component.Render(r.Context(), w)
		return
	}

	// Create session
	session, err := h.sessionManager.CreateSession(user.ID)
	if err != nil {
		h.loginError(w, r, "Failed to create session")
		return
	}

	// Store session token in cookie
	if err := h.middleware.SetSessionToken(w, r, session.ID); err != nil {
		h.loginError(w, r, "Failed to save session")
		return
	}

	// Record successful login
	_ = h.authStore.RecordLoginAttempt(ip, email, true)
	h.rateLimiter.ResetIP(ip)

	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

// HandleLogout logs the user out.
func (h *AuthHandlers) HandleLogout(w http.ResponseWriter, r *http.Request) {
	// Get current session token and delete it from the database
	session := h.middleware.GetSession(r)
	if token, ok := session.Values["token"].(string); ok && token != "" {
		_ = h.sessionManager.DeleteSession(token)
	}

	// Clear the cookie
	_ = h.middleware.ClearSession(w, r)

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (h *AuthHandlers) loginError(w http.ResponseWriter, r *http.Request, message string) {
	data := templates.PageData{
		Title:      "Login",
		FlashError: message,
	}
	component := templates.LoginPage(data, false)
	_ = component.Render(r.Context(), w)
}
