package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"

	"rillnet/pkg/config"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// rateLimiterStore stores per-key (for example, per IP) rate limiters.
type rateLimiterStore struct {
	mu        sync.Mutex
	limiters  map[string]*rate.Limiter
	rate      rate.Limit
	burstSize int
}

func newRateLimiterStore(r rate.Limit, burst int) *rateLimiterStore {
	return &rateLimiterStore{
		limiters:  make(map[string]*rate.Limiter),
		rate:      r,
		burstSize: burst,
	}
}

func (s *rateLimiterStore) getLimiter(key string) *rate.Limiter {
	s.mu.Lock()
	defer s.mu.Unlock()

	limiter, exists := s.limiters[key]
	if !exists {
		limiter = rate.NewLimiter(s.rate, s.burstSize)
		s.limiters[key] = limiter
	}
	return limiter
}

// clientIP extracts the IP part from the request's remote address.
func clientIP(r *http.Request) string {
	// Try X-Forwarded-For first (behind proxies)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := net.ParseIP(xff)
		if parts != nil {
			return parts.String()
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// NewHTTPRateLimitMiddleware returns Gin middleware that applies simple IP-based rate limiting.
func NewHTTPRateLimitMiddleware(cfg *config.Config) gin.HandlerFunc {
	if !cfg.RateLimiting.Enabled {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	rps := cfg.RateLimiting.HTTP.RequestsPerSecond
	burst := cfg.RateLimiting.HTTP.Burst

	store := newRateLimiterStore(rate.Limit(rps), burst)

	var globalSem chan struct{}
	if cfg.RateLimiting.HTTP.MaxConcurrent > 0 {
		globalSem = make(chan struct{}, cfg.RateLimiting.HTTP.MaxConcurrent)
	}

	return func(c *gin.Context) {
		// Global concurrent requests throttling
		if globalSem != nil {
			select {
			case globalSem <- struct{}{}:
				defer func() { <-globalSem }()
			default:
				c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
					"error": "too many concurrent requests",
				})
				return
			}
		}

		ip := clientIP(c.Request)
		limiter := store.getLimiter(ip)
		if !limiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate limit exceeded",
				"retry_after": int(time.Second),
			})
			return
		}
		c.Next()
	}
}


