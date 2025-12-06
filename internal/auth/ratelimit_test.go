package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRateLimiter(t *testing.T) {
	t.Run("allows requests within limit", func(t *testing.T) {
		rl := NewRateLimiter(RateLimitConfig{
			RequestsPerMinute: 60, // 1 per second
			BurstSize:         5,
			CleanupInterval:   time.Hour, // Long interval for test
			MaxAge:            time.Hour,
		})

		// First few requests should be allowed (burst)
		for i := 0; i < 5; i++ {
			assert.True(t, rl.Allow("test-key"), "Request %d should be allowed", i)
		}
	})

	t.Run("blocks requests over limit", func(t *testing.T) {
		rl := NewRateLimiter(RateLimitConfig{
			RequestsPerMinute: 60, // 1 per second
			BurstSize:         3,
			CleanupInterval:   time.Hour,
			MaxAge:            time.Hour,
		})

		// Exhaust the burst
		for i := 0; i < 3; i++ {
			rl.Allow("test-key")
		}

		// Next request should be blocked
		assert.False(t, rl.Allow("test-key"))
	})

	t.Run("different keys have separate limits", func(t *testing.T) {
		rl := NewRateLimiter(RateLimitConfig{
			RequestsPerMinute: 60,
			BurstSize:         3,
			CleanupInterval:   time.Hour,
			MaxAge:            time.Hour,
		})

		// Exhaust key1
		for i := 0; i < 3; i++ {
			rl.Allow("key1")
		}
		assert.False(t, rl.Allow("key1"))

		// key2 should still be allowed
		assert.True(t, rl.Allow("key2"))
	})

	t.Run("reset clears limiter for key", func(t *testing.T) {
		rl := NewRateLimiter(RateLimitConfig{
			RequestsPerMinute: 60,
			BurstSize:         3,
			CleanupInterval:   time.Hour,
			MaxAge:            time.Hour,
		})

		// Exhaust the burst
		for i := 0; i < 3; i++ {
			rl.Allow("reset-key")
		}
		assert.False(t, rl.Allow("reset-key"))

		// Reset the key
		rl.Reset("reset-key")

		// Should be allowed again
		assert.True(t, rl.Allow("reset-key"))
	})

	t.Run("size returns number of tracked keys", func(t *testing.T) {
		rl := NewRateLimiter(RateLimitConfig{
			RequestsPerMinute: 60,
			BurstSize:         5,
			CleanupInterval:   time.Hour,
			MaxAge:            time.Hour,
		})

		assert.Equal(t, 0, rl.Size())

		rl.Allow("key1")
		assert.Equal(t, 1, rl.Size())

		rl.Allow("key2")
		assert.Equal(t, 2, rl.Size())

		rl.Allow("key1") // Same key, no increase
		assert.Equal(t, 2, rl.Size())
	})
}

func TestIPRateLimiter(t *testing.T) {
	t.Run("limits by IP address", func(t *testing.T) {
		rl := NewIPRateLimiter(60, 3)

		// Exhaust IP1
		for i := 0; i < 3; i++ {
			rl.AllowIP("192.168.1.1")
		}
		assert.False(t, rl.AllowIP("192.168.1.1"))

		// IP2 should still be allowed
		assert.True(t, rl.AllowIP("192.168.1.2"))
	})

	t.Run("reset works for IP", func(t *testing.T) {
		rl := NewIPRateLimiter(60, 2)

		// Exhaust the IP
		rl.AllowIP("10.0.0.1")
		rl.AllowIP("10.0.0.1")
		assert.False(t, rl.AllowIP("10.0.0.1"))

		// Reset
		rl.ResetIP("10.0.0.1")

		// Should work again
		assert.True(t, rl.AllowIP("10.0.0.1"))
	})
}

func TestDefaultRateLimitConfig(t *testing.T) {
	config := DefaultRateLimitConfig()

	assert.Equal(t, 5, config.RequestsPerMinute)
	assert.Equal(t, 10, config.BurstSize)
	assert.Equal(t, 5*time.Minute, config.CleanupInterval)
	assert.Equal(t, 30*time.Minute, config.MaxAge)
}
