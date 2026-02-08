package api

import (
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
)

// visitor tracks the rate limit state for a single IP.
type visitor struct {
	tokens    float64
	lastCheck time.Time
}

// RateLimiter is a per-IP token-bucket rate limiter.
type RateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rate     float64 // tokens per second
	burst    int     // max tokens
}

// NewRateLimiter creates a rate limiter with the given rate (requests/sec) and burst size.
func NewRateLimiter(rps float64, burst int) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		rate:     rps,
		burst:    burst,
	}

	// Clean up stale entries every 5 minutes
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			rl.cleanup()
		}
	}()

	return rl
}

// Middleware returns an echo middleware function that enforces rate limits.
func (rl *RateLimiter) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ip := c.RealIP()
			if !rl.allow(ip) {
				slog.Warn("rate limit exceeded", "ip", ip)
				return c.JSON(http.StatusTooManyRequests, map[string]string{
					"error": "rate limit exceeded, try again later",
				})
			}
			return next(c)
		}
	}
}

func (rl *RateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[ip]
	now := time.Now()

	if !exists {
		rl.visitors[ip] = &visitor{
			tokens:    float64(rl.burst) - 1,
			lastCheck: now,
		}
		return true
	}

	// Add tokens based on elapsed time
	elapsed := now.Sub(v.lastCheck).Seconds()
	v.tokens += elapsed * rl.rate
	if v.tokens > float64(rl.burst) {
		v.tokens = float64(rl.burst)
	}
	v.lastCheck = now

	if v.tokens < 1 {
		return false
	}

	v.tokens--
	return true
}

func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-10 * time.Minute)
	for ip, v := range rl.visitors {
		if v.lastCheck.Before(cutoff) {
			delete(rl.visitors, ip)
		}
	}
}

// RequestLogger returns an echo middleware that logs requests using slog.
func RequestLogger() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			err := next(c)

			req := c.Request()
			res := c.Response()

			slog.Info("request",
				"method", req.Method,
				"path", req.URL.Path,
				"status", res.Status,
				"latency_ms", time.Since(start).Milliseconds(),
				"ip", c.RealIP(),
				"user_agent", req.UserAgent(),
				"bytes_out", res.Size,
			)

			return err
		}
	}
}
