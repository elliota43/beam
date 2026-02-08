package api

import (
	"beam/internal/server/config"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// SetupRouter creates and configures the echo router with all routes and middleware.
func SetupRouter(handler *Handler, cfg *config.Config) *echo.Echo {
	e := echo.New()
	e.HideBanner = true

	// Global middleware
	e.Use(middleware.Recover())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Content-Type", "Authorization"},
	}))
	e.Use(RequestLogger())

	// Rate limiter on upload endpoint only
	uploadLimiter := NewRateLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst)

	// Health & stats
	e.GET("/health", handler.HandleHealth)
	e.GET("/api/stats", handler.HandleStats)

	// Upload (rate-limited)
	e.POST("/api/upload", handler.HandleUpload, uploadLimiter.Middleware())

	// Download
	e.GET("/d/:id", handler.HandleDownload)

	// Info
	e.GET("/api/info/:id", handler.HandleInfo)

	// Delete
	e.DELETE("/api/delete/:id/:token", handler.HandleDelete)

	return e
}
