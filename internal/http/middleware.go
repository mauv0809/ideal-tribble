package http

import (
	"context"
	"io"
	"net/http"

	"github.com/charmbracelet/log"
	"github.com/slack-go/slack"
)

// Middleware defines the standard signature for an HTTP middleware.
type Middleware func(http.Handler) http.Handler

// Chain combines multiple middlewares into a single handler.
// The middlewares are applied in the order they are passed.
func Chain(h http.Handler, middlewares ...Middleware) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}

// contextKey is a custom type to avoid key collisions in context.
type contextKey string

const (
	dryRunKey contextKey = "dryRun"
)

// paramsMiddleware handles common query parameters like 'verbose' and 'dry_run'.
func paramsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Info("incoming request", "method", r.Method, "url", r.URL.String())
		// Handle 'verbose' for request-scoped verbose logging.
		if r.URL.Query().Get("verbose") == "true" {
			originalLevel := log.GetLevel()
			log.SetLevel(log.DebugLevel)
			// This defer will reset the log level after the handler finishes.
			// Note: For long-running background tasks spawned by a handler (like /check),
			// this will not keep the log level verbose for the entire background task.
			defer log.SetLevel(originalLevel)
		}

		// Handle 'dry_run' and add it to the request context.
		isDryRun := r.URL.Query().Get("dry_run") == "true"
		ctx := context.WithValue(r.Context(), dryRunKey, isDryRun)

		// Call the next handler with the modified context.
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// isDryRunFromContext is a helper to safely retrieve the dry_run flag from the request context.
func isDryRunFromContext(r *http.Request) bool {
	dryRun, ok := r.Context().Value(dryRunKey).(bool)
	return ok && dryRun
}

// VerifySlackSignature is a middleware that verifies the Slack request signature.
func (s *Server) VerifySlackSignature(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		verifier, err := slack.NewSecretsVerifier(r.Header, s.Cfg.Slack.SigningSecret)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// TeeReader copies the reader's input to the writer and returns a new Reader.
		// This allows us to read the request body for signature verification without consuming it for the next handler.
		r.Body = io.NopCloser(io.TeeReader(r.Body, &verifier))

		// Call the next handler in the chain to process the request
		next.ServeHTTP(w, r)

		// After the handler has processed the request, ensure the signature
		if err = verifier.Ensure(); err != nil {
			http.Error(w, "Unauthorized: Slack signature verification failed", http.StatusUnauthorized)
			return
		}
	})
}
