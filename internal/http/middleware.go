package http

import (
	"context"
	"io"
	"net/http"
	"strings"

	"github.com/charmbracelet/log"
	handlers "github.com/mauv0809/ideal-tribble/internal/http/handlers"
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
		ctx := context.WithValue(r.Context(), handlers.DryRunKey, isDryRun)

		// Call the next handler with the modified context.
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// VerifySlackSignature is a middleware that verifies the Slack request signature.
func (s *Server) VerifySlackSignature(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		verifier, err := slack.NewSecretsVerifier(r.Header, s.Cfg.Slack.SigningSecret)
		if err != nil {
			log.Error("failed to create secrets verifier", "error", err)

			// Return 401 if error relates to signature or headers
			if strings.Contains(err.Error(), "missing headers") ||
				strings.Contains(err.Error(), "invalid byte") ||
				strings.Contains(err.Error(), "timestamp is too old") {
				http.Error(w, "Unauthorized: Slack signature verification failed", http.StatusUnauthorized)
			} else {
				// For unknown/unexpected errors, return 500
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
			return
		}

		// Read the entire body to verify the signature before proceeding
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Error("failed to read request body", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Write the body to the verifier for signature calculation
		if _, err := verifier.Write(body); err != nil {
			log.Error("failed to write body to verifier", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Verify the signature BEFORE calling the handler
		if err = verifier.Ensure(); err != nil {
			log.Error("Slack signature verification failed", "error", err)
			http.Error(w, "Unauthorized: Slack signature verification failed", http.StatusUnauthorized)
			return
		}

		// Restore the body for the handler to read
		r.Body = io.NopCloser(strings.NewReader(string(body)))

		next.ServeHTTP(w, r)
	})
}
