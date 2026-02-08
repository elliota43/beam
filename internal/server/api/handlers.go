package api

import (
	"errors"
	"fmt"
	"net/http"

	"beam/internal/server/database"
	"beam/internal/server/service"

	"github.com/labstack/echo/v4"
)

// Handler contains the HTTP handlers for the Beam API.
type Handler struct {
	svc *service.UploadService
	db  *database.DB
}

// NewHandler creates a new handler with the given service dependency.
func NewHandler(svc *service.UploadService, db *database.DB) *Handler {
	return &Handler{svc: svc, db: db}
}

// HandleUpload handles POST /api/upload.
// Accepts a multipart form with a "file" field and optional "password" field.
func (h *Handler) HandleUpload(c echo.Context) error {
	// Read the uploaded file from the multipart form
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "file is required (use form field 'file')",
		})
	}

	src, err := fileHeader.Open()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": "failed to read uploaded file",
		})
	}
	defer src.Close()

	password := c.FormValue("password")

	result, err := h.svc.ProcessUpload(
		c.Request().Context(),
		fileHeader.Filename,
		src,
		fileHeader.Size,
		password,
	)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusCreated, result)
}

// HandleDownload handles GET /d/:id.
// Serves the file as an attachment. Accepts an optional "password" query param.
func (h *Handler) HandleDownload(c echo.Context) error {
	id := c.Param("id")
	password := c.QueryParam("password")

	filePath, filename, err := h.svc.Download(c.Request().Context(), id, password)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.Attachment(filePath, filename)
}

// HandleInfo handles GET /api/info/:id.
// Returns upload metadata without serving the file.
func (h *Handler) HandleInfo(c echo.Context) error {
	id := c.Param("id")

	info, err := h.svc.GetInfo(c.Request().Context(), id)
	if err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, info)
}

// HandleDelete handles DELETE /api/delete/:id/:token.
// Deletes an upload using the deletion token provided at upload time.
func (h *Handler) HandleDelete(c echo.Context) error {
	id := c.Param("id")
	token := c.Param("token")

	if err := h.svc.DeleteUpload(c.Request().Context(), id, token); err != nil {
		return mapServiceError(c, err)
	}

	return c.JSON(http.StatusOK, echo.Map{
		"message": "upload deleted successfully",
	})
}

// HandleHealth handles GET /health.
// Returns the health status of the server, including database connectivity.
func (h *Handler) HandleHealth(c echo.Context) error {
	status := "healthy"
	dbStatus := "connected"

	if err := h.db.HealthCheck(c.Request().Context()); err != nil {
		status = "degraded"
		dbStatus = fmt.Sprintf("error: %v", err)
	}

	return c.JSON(http.StatusOK, echo.Map{
		"status":   status,
		"database": dbStatus,
	})
}

// HandleStats handles GET /api/stats.
// Returns aggregate server statistics.
func (h *Handler) HandleStats(c echo.Context) error {
	stats, err := h.svc.GetStats(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": "failed to retrieve stats",
		})
	}

	return c.JSON(http.StatusOK, echo.Map{
		"total_uploads":      stats.TotalUploads,
		"active_uploads":     stats.ActiveUploads,
		"total_downloads":    stats.TotalDownloads,
		"storage_used_bytes": stats.StorageUsed,
		"storage_used_human": humanizeBytes(stats.StorageUsed),
	})
}

// mapServiceError translates service-layer errors into appropriate HTTP responses.
func mapServiceError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, service.ErrNotFound):
		return c.JSON(http.StatusNotFound, echo.Map{"error": "upload not found"})
	case errors.Is(err, service.ErrExpired):
		return c.JSON(http.StatusGone, echo.Map{"error": "upload has expired"})
	case errors.Is(err, service.ErrPasswordRequired):
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "password_required"})
	case errors.Is(err, service.ErrInvalidPassword):
		return c.JSON(http.StatusForbidden, echo.Map{"error": "invalid password"})
	case errors.Is(err, service.ErrInvalidToken):
		return c.JSON(http.StatusForbidden, echo.Map{"error": "invalid deletion token"})
	case errors.Is(err, service.ErrFileTooLarge):
		return c.JSON(http.StatusRequestEntityTooLarge, echo.Map{
			"error": "file exceeds maximum allowed size",
		})
	case errors.Is(err, service.ErrInvalidZip):
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid or corrupt ZIP file"})
	case errors.Is(err, service.ErrDangerousFile):
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	default:
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "internal server error"})
	}
}

// humanizeBytes formats a byte count into a human-readable string.
func humanizeBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
