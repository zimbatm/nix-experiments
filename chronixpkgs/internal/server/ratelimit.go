package server

import (
	"net/http"
	"sync"
	"time"

	"github.com/zimbatm/nix-experiments/chronixpkgs/internal/utils"
	"golang.org/x/time/rate"
)

// RateLimiter manages per-IP rate limiting
type RateLimiter struct {
	limiters    map[string]*rate.Limiter
	mu          sync.RWMutex
	rate        int
	burst       int
	cleanupStop chan struct{}
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(requestsPerMinute, burstSize int) *RateLimiter {
	rl := &RateLimiter{
		limiters:    make(map[string]*rate.Limiter),
		rate:        requestsPerMinute,
		burst:       burstSize,
		cleanupStop: make(chan struct{}),
	}

	// Start cleanup routine
	go rl.cleanup()

	return rl
}

// Allow checks if a request from the given IP is allowed
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.RLock()
	limiter, exists := rl.limiters[ip]
	rl.mu.RUnlock()

	if !exists {
		// Create new limiter
		limiter = rate.NewLimiter(rate.Limit(float64(rl.rate)/60.0), rl.burst)
		rl.mu.Lock()
		rl.limiters[ip] = limiter
		rl.mu.Unlock()
	}

	return limiter.Allow()
}

// cleanup removes inactive rate limiters periodically
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			// Remove limiters that haven't been used recently
			// This is a simple implementation - could be improved with last-used tracking
			if len(rl.limiters) > 1000 { // Only cleanup if we have many limiters
				// Clear all limiters (simple approach)
				rl.limiters = make(map[string]*rate.Limiter)
			}
			rl.mu.Unlock()
		case <-rl.cleanupStop:
			return
		}
	}
}

// Stop gracefully stops the rate limiter
func (rl *RateLimiter) Stop() {
	close(rl.cleanupStop)
}

// Middleware returns an HTTP middleware that enforces rate limiting
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := utils.GetClientIP(r)

		if !rl.Allow(ip) {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}
