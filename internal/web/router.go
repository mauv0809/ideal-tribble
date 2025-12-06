package web

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/mauv0809/ideal-tribble/internal/auth"
	"github.com/mauv0809/ideal-tribble/internal/pairings"
)

//go:embed static/*
var staticFiles embed.FS

// Config holds the configuration for the web server.
type Config struct {
	SessionSecret    []byte
	TOTPEncryptionKey []byte
}

// Router sets up all web routes.
type Router struct {
	mux             *http.ServeMux
	middleware      *Middleware
	authHandlers    *AuthHandlers
	handlers        *Handlers
	profileHandlers *ProfileHandlers
	adminHandlers   *AdminHandlers
}

// NewRouter creates a new web router with all handlers configured.
func NewRouter(
	config Config,
	authStore *auth.Store,
	sessionManager *auth.SessionManager,
	pairingsStore pairings.PairingsStore,
	fetchService ...FetchService,
) (*Router, error) {
	// Create rate limiter
	rateLimiter := auth.NewIPRateLimiter(5, 10) // 5 requests/min, burst of 10

	// Create TOTP manager
	totpManager, err := auth.NewTOTPManager(config.TOTPEncryptionKey)
	if err != nil {
		return nil, err
	}

	// Create middleware
	middleware := NewMiddleware(config.SessionSecret, authStore, sessionManager, rateLimiter)

	// Get fetch service if provided
	var fs FetchService
	if len(fetchService) > 0 {
		fs = fetchService[0]
	}

	// Create handlers
	authHandlers := NewAuthHandlers(middleware, authStore, sessionManager, totpManager, rateLimiter)
	handlers := NewHandlers(middleware, pairingsStore, fs)
	profileHandlers := NewProfileHandlers(middleware, authStore, sessionManager, totpManager)
	adminHandlers := NewAdminHandlers(middleware, authStore)

	router := &Router{
		mux:             http.NewServeMux(),
		middleware:      middleware,
		authHandlers:    authHandlers,
		handlers:        handlers,
		profileHandlers: profileHandlers,
		adminHandlers:   adminHandlers,
	}

	router.setupRoutes()
	return router, nil
}

func (r *Router) setupRoutes() {
	// Static files
	staticFS, _ := fs.Sub(staticFiles, "static")
	r.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// Public routes
	r.mux.HandleFunc("GET /login", r.authHandlers.LoginPage)
	r.mux.HandleFunc("POST /login", r.authHandlers.HandleLogin)
	r.mux.HandleFunc("POST /login/totp", r.authHandlers.HandleTOTPLogin)
	r.mux.HandleFunc("POST /logout", r.authHandlers.HandleLogout)

	// Redirect root to dashboard or login
	r.mux.HandleFunc("GET /", func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/" {
			http.NotFound(w, req)
			return
		}
		http.Redirect(w, req, "/dashboard", http.StatusSeeOther)
	})

	// Protected routes - wrapped with auth middleware
	r.mux.Handle("GET /dashboard", r.middleware.RequireAuth(http.HandlerFunc(r.handlers.Dashboard)))
	r.mux.Handle("POST /fetch-matches", r.middleware.RequireAuth(http.HandlerFunc(r.handlers.FetchMatches)))

	// Pairings routes
	r.mux.Handle("GET /pairings", r.middleware.RequireAuth(http.HandlerFunc(r.handlers.PairingsList)))
	r.mux.Handle("GET /pairings/new", r.middleware.RequireAuth(http.HandlerFunc(r.handlers.NewPairingPage)))
	r.mux.Handle("POST /pairings", r.middleware.RequireAuth(http.HandlerFunc(r.handlers.CreatePairing)))
	r.mux.Handle("GET /pairings/{id}", r.middleware.RequireAuth(http.HandlerFunc(r.handlers.PairingDetail)))
	r.mux.Handle("GET /pairings/{id}/matches", r.middleware.RequireAuth(http.HandlerFunc(r.handlers.MatchHistoryPartial)))
	r.mux.Handle("DELETE /pairings/{id}", r.middleware.RequireAuth(http.HandlerFunc(r.handlers.DeletePairing)))
	r.mux.Handle("PATCH /pairings/{id}/activate", r.middleware.RequireAuth(http.HandlerFunc(r.handlers.ActivatePairing)))
	r.mux.Handle("PATCH /pairings/{id}/deactivate", r.middleware.RequireAuth(http.HandlerFunc(r.handlers.DeactivatePairing)))

	// Analytics routes
	r.mux.Handle("GET /pairings/{id}/opponents", r.middleware.RequireAuth(http.HandlerFunc(r.handlers.OpponentStats)))
	r.mux.Handle("GET /pairings/{id}/opponents/{opp1}/{opp2}", r.middleware.RequireAuth(http.HandlerFunc(r.handlers.OpponentDetail)))
	r.mux.Handle("GET /pairings/{id}/players", r.middleware.RequireAuth(http.HandlerFunc(r.handlers.IndividualPlayers)))
	r.mux.Handle("GET /pairings/{id}/players/{playerID}", r.middleware.RequireAuth(http.HandlerFunc(r.handlers.IndividualPlayerDetail)))

	// Match detail route
	r.mux.Handle("GET /matches/{matchID}", r.middleware.RequireAuth(http.HandlerFunc(r.handlers.MatchDetail)))

	// Profile routes
	r.mux.Handle("GET /profile", r.middleware.RequireAuth(http.HandlerFunc(r.profileHandlers.ProfilePage)))
	r.mux.Handle("POST /profile/password", r.middleware.RequireAuth(http.HandlerFunc(r.profileHandlers.ChangePassword)))
	r.mux.Handle("GET /profile/totp/setup", r.middleware.RequireAuth(http.HandlerFunc(r.profileHandlers.TOTPSetupPage)))
	r.mux.Handle("GET /profile/totp/qr", r.middleware.RequireAuth(http.HandlerFunc(r.profileHandlers.TOTPQRCode)))
	r.mux.Handle("POST /profile/totp/verify", r.middleware.RequireAuth(http.HandlerFunc(r.profileHandlers.VerifyTOTP)))
	r.mux.Handle("POST /profile/totp/disable", r.middleware.RequireAuth(http.HandlerFunc(r.profileHandlers.DisableTOTP)))

	// Admin routes - require auth + admin
	adminMiddleware := func(h http.Handler) http.Handler {
		return r.middleware.RequireAuth(r.middleware.RequireAdmin(h))
	}
	r.mux.Handle("GET /admin/users", adminMiddleware(http.HandlerFunc(r.adminHandlers.UserListPage)))
	r.mux.Handle("GET /admin/users/new", adminMiddleware(http.HandlerFunc(r.adminHandlers.NewUserPage)))
	r.mux.Handle("POST /admin/users", adminMiddleware(http.HandlerFunc(r.adminHandlers.CreateUser)))
	r.mux.Handle("DELETE /admin/users/{id}", adminMiddleware(http.HandlerFunc(r.adminHandlers.DeleteUser)))
}

// ServeHTTP implements the http.Handler interface.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}
