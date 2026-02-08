package storage

import (
	"context"
	"log/slog"
	"time"

	"beam/internal/server/database"
)

// CleanupService periodically removes expired uploads from both
// the database and file storage.
type CleanupService struct {
	repo     *database.Repository
	store    Store
	interval time.Duration
	done     chan struct{}
}

// NewCleanupService creates a new cleanup service.
func NewCleanupService(repo *database.Repository, store Store, interval time.Duration) *CleanupService {
	return &CleanupService{
		repo:     repo,
		store:    store,
		interval: interval,
		done:     make(chan struct{}),
	}
}

// Start begins the cleanup loop in a background goroutine.
func (cs *CleanupService) Start(ctx context.Context) {
	slog.Info("cleanup service started", "interval", cs.interval)

	go func() {
		ticker := time.NewTicker(cs.interval)
		defer ticker.Stop()

		// Run once immediately on start
		cs.runCleanup(ctx)

		for {
			select {
			case <-ticker.C:
				cs.runCleanup(ctx)
			case <-ctx.Done():
				slog.Info("cleanup service stopping")
				close(cs.done)
				return
			}
		}
	}()
}

// Wait blocks until the cleanup service has fully stopped.
func (cs *CleanupService) Wait() {
	<-cs.done
}

func (cs *CleanupService) runCleanup(ctx context.Context) {
	slog.Info("running cleanup cycle")

	expired, err := cs.repo.GetExpired(ctx)
	if err != nil {
		slog.Error("failed to get expired uploads", "error", err)
		return
	}

	if len(expired) == 0 {
		slog.Info("no expired uploads to clean up")
		return
	}

	var cleaned, failed int
	for _, upload := range expired {
		// Delete file from storage
		if err := cs.store.Delete(upload.ID); err != nil {
			slog.Error("failed to delete file",
				"upload_id", upload.ID,
				"error", err,
			)
			failed++
			continue
		}

		// Delete record from database
		if err := cs.repo.Delete(ctx, upload.ID); err != nil {
			slog.Error("failed to delete db record",
				"upload_id", upload.ID,
				"error", err,
			)
			failed++
			continue
		}

		cleaned++
		slog.Info("cleaned up expired upload",
			"upload_id", upload.ID,
			"filename", upload.Filename,
			"expired_at", upload.ExpiresAt,
		)
	}

	slog.Info("cleanup cycle complete",
		"cleaned", cleaned,
		"failed", failed,
		"total_expired", len(expired),
	)
}
