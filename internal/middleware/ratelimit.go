package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/cherif-safephone/safephone-backend/internal/cache"
	"github.com/cherif-safephone/safephone-backend/internal/domain"
	"github.com/cherif-safephone/safephone-backend/internal/respond"
)

// RateLimiter provides Redis-based sliding window rate limiting.
type RateLimiter struct {
	cache      *cache.Client
	maxPerMin  int
	windowSize time.Duration
}

// NewRateLimiter creates a rate limiter with the given requests-per-minute limit.
func NewRateLimiter(c *cache.Client, maxPerMin int) *RateLimiter {
	return &RateLimiter{
		cache:      c,
		maxPerMin:  maxPerMin,
		windowSize: 1 * time.Minute,
	}
}

// Limit returns middleware that rate-limits requests by client IP.
func (rl *RateLimiter) Limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := fmt.Sprintf("ratelimit:%s", r.RemoteAddr)

		count, err := rl.cache.Incr(r.Context(), key, rl.windowSize)
		if err != nil {
			// On Redis failure, allow the request through (fail open for rate limiting)
			next.ServeHTTP(w, r)
			return
		}

		if count > int64(rl.maxPerMin) {
			w.Header().Set("Retry-After", "60")
			respond.Error(w, r, domain.RateLimited())
			return
		}

		next.ServeHTTP(w, r)
	})
}
