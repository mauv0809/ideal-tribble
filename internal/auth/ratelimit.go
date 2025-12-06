package auth

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter manages rate limiting for login attempts.
type RateLimiter struct {
	mu       sync.RWMutex
	limiters map[string]*rate.Limiter

	// Configuration
	requestsPerMinute int
	burstSize         int
	cleanupInterval   time.Duration
	maxAge            time.Duration
}

type limiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimitConfig holds rate limiter configuration.
type RateLimitConfig struct {
	RequestsPerMinute int           // Allowed requests per minute
	BurstSize         int           // Maximum burst size
	CleanupInterval   time.Duration // How often to clean up old entries
	MaxAge            time.Duration // Remove entries older than this
}

// DefaultRateLimitConfig returns sensible defaults.
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		RequestsPerMinute: 5,
		BurstSize:         10,
		CleanupInterval:   5 * time.Minute,
		MaxAge:            30 * time.Minute,
	}
}

// NewRateLimiter creates a new rate limiter with the given configuration.
func NewRateLimiter(config RateLimitConfig) *RateLimiter {
	rl := &RateLimiter{
		limiters:          make(map[string]*rate.Limiter),
		requestsPerMinute: config.RequestsPerMinute,
		burstSize:         config.BurstSize,
		cleanupInterval:   config.CleanupInterval,
		maxAge:            config.MaxAge,
	}

	// Start cleanup goroutine
	go rl.cleanupLoop()

	return rl
}

// Allow checks if a request from the given key (e.g., IP address) is allowed.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	limiter, exists := rl.limiters[key]
	if !exists {
		// Create a new limiter: requestsPerMinute / 60 = requests per second
		limiter = rate.NewLimiter(rate.Limit(float64(rl.requestsPerMinute)/60.0), rl.burstSize)
		rl.limiters[key] = limiter
	}
	rl.mu.Unlock()

	return limiter.Allow()
}

// Reset removes the rate limiter for a specific key.
// Useful after successful authentication.
func (rl *RateLimiter) Reset(key string) {
	rl.mu.Lock()
	delete(rl.limiters, key)
	rl.mu.Unlock()
}

// cleanupLoop periodically removes old rate limiter entries.
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		rl.cleanup()
	}
}

// cleanup removes entries that haven't been used recently.
// Note: This is a simple implementation that removes all entries periodically.
// A more sophisticated version would track last access time per entry.
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Simple approach: clear all limiters periodically
	// This is acceptable because rate.Limiter will be recreated on next access
	// and the token bucket will start fresh.
	// For a high-traffic system, you'd want to track last access time.
	if len(rl.limiters) > 1000 {
		rl.limiters = make(map[string]*rate.Limiter)
	}
}

// Size returns the current number of tracked limiters.
func (rl *RateLimiter) Size() int {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	return len(rl.limiters)
}

// IPRateLimiter provides rate limiting specifically for IP addresses.
type IPRateLimiter struct {
	*RateLimiter
}

// NewIPRateLimiter creates a rate limiter configured for IP-based rate limiting.
func NewIPRateLimiter(requestsPerMinute, burstSize int) *IPRateLimiter {
	return &IPRateLimiter{
		RateLimiter: NewRateLimiter(RateLimitConfig{
			RequestsPerMinute: requestsPerMinute,
			BurstSize:         burstSize,
			CleanupInterval:   5 * time.Minute,
			MaxAge:            30 * time.Minute,
		}),
	}
}

// AllowIP checks if a request from the given IP address is allowed.
func (rl *IPRateLimiter) AllowIP(ipAddress string) bool {
	return rl.Allow(ipAddress)
}

// ResetIP resets the rate limit for a specific IP address.
func (rl *IPRateLimiter) ResetIP(ipAddress string) {
	rl.Reset(ipAddress)
}
