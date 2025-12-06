package web

import (
	"context"
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/mauv0809/ideal-tribble/internal/auth"
)

type contextKey string

const (
	userContextKey    contextKey = "user"
	sessionContextKey contextKey = "session"
)

// Middleware provides HTTP middleware for authentication.
type Middleware struct {
	store          *sessions.CookieStore
	authStore      *auth.Store
	sessionManager *auth.SessionManager
	rateLimiter    *auth.IPRateLimiter
}

// NewMiddleware creates a new middleware instance.
func NewMiddleware(
	sessionSecret []byte,
	authStore *auth.Store,
	sessionManager *auth.SessionManager,
	rateLimiter *auth.IPRateLimiter,
) *Middleware {
	store := sessions.NewCookieStore(sessionSecret)
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400, // 24 hours
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteLaxMode,
	}

	return &Middleware{
		store:          store,
		authStore:      authStore,
		sessionManager: sessionManager,
		rateLimiter:    rateLimiter,
	}
}

// RequireAuth ensures the user is authenticated.
func (m *Middleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := m.getUserFromSession(r)
		if user == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// Add user to context
		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAdmin ensures the user is an admin.
func (m *Middleware) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := GetUser(r)
		if user == nil || !user.IsAdmin {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RateLimit applies rate limiting to a handler.
func (m *Middleware) RateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := getClientIP(r)
		if !m.rateLimiter.AllowIP(ip) {
			http.Error(w, "Too many requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// getUserFromSession retrieves the authenticated user from the session.
func (m *Middleware) getUserFromSession(r *http.Request) *auth.User {
	session, err := m.store.Get(r, "session")
	if err != nil {
		return nil
	}

	token, ok := session.Values["token"].(string)
	if !ok || token == "" {
		return nil
	}

	userID, err := m.sessionManager.ValidateSession(token)
	if err != nil {
		return nil
	}

	user, err := m.authStore.GetUserByID(userID)
	if err != nil {
		return nil
	}

	// Extend the session (sliding window)
	_ = m.sessionManager.ExtendSession(token)

	return user
}

// GetSession returns the session for the current request.
func (m *Middleware) GetSession(r *http.Request) *sessions.Session {
	session, _ := m.store.Get(r, "session")
	return session
}

// SetSessionToken stores the session token in the cookie.
func (m *Middleware) SetSessionToken(w http.ResponseWriter, r *http.Request, token string) error {
	session, _ := m.store.Get(r, "session")
	session.Values["token"] = token
	return session.Save(r, w)
}

// ClearSession removes the session.
func (m *Middleware) ClearSession(w http.ResponseWriter, r *http.Request) error {
	session, _ := m.store.Get(r, "session")
	session.Values["token"] = ""
	session.Options.MaxAge = -1
	return session.Save(r, w)
}

// SetFlash sets a flash message in the session.
func (m *Middleware) SetFlash(w http.ResponseWriter, r *http.Request, key, value string) error {
	session, _ := m.store.Get(r, "flash")
	session.AddFlash(value, key)
	return session.Save(r, w)
}

// GetFlash retrieves and clears flash messages.
func (m *Middleware) GetFlash(w http.ResponseWriter, r *http.Request, key string) []string {
	session, _ := m.store.Get(r, "flash")
	flashes := session.Flashes(key)
	_ = session.Save(r, w)

	result := make([]string, len(flashes))
	for i, f := range flashes {
		result[i], _ = f.(string)
	}
	return result
}

// GetUser retrieves the user from the request context.
func GetUser(r *http.Request) *auth.User {
	user, _ := r.Context().Value(userContextKey).(*auth.User)
	return user
}

// getClientIP extracts the client IP from the request.
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (for reverse proxies)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the chain
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return xff[:i]
			}
		}
		return xff
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	// Remove port from address
	addr := r.RemoteAddr
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i]
		}
	}
	return addr
}
