package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"rillnet/pkg/config"

	"github.com/gin-gonic/gin"
)

// Test that when rate limiting is disabled, middleware lets all requests through.
func TestHTTPRateLimitMiddleware_Disabled_AllowsRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := config.DefaultConfig()
	cfg.RateLimiting.Enabled = false

	router := gin.New()
	router.Use(NewHTTPRateLimitMiddleware(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w1.Code)
	}

	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected status 200 on second request, got %d", w2.Code)
	}
}

// Test basic per-IP rate limiting behaviour.
func TestHTTPRateLimitMiddleware_Enabled_RateLimited(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := config.DefaultConfig()
	cfg.RateLimiting.Enabled = true
	cfg.RateLimiting.HTTP.RequestsPerSecond = 1
	cfg.RateLimiting.HTTP.Burst = 1
	cfg.RateLimiting.HTTP.MaxConcurrent = 0

	router := gin.New()
	router.Use(NewHTTPRateLimitMiddleware(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// First request should pass.
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("expected status 200 for first request, got %d", w1.Code)
	}

	// Second immediate request from same "IP" should be limited.
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status 429 for second request, got %d", w2.Code)
	}
}


