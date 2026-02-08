package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

var (
	ErrUploadNotFound = errors.New("upload not found")
)

// Repository provides CRUD operations for uploads.
type Repository struct {
	db *DB
}

// NewRepository creates a new Repository.
func NewRepository(db *DB) *Repository {
	return &Repository{db: db}
}

// Create inserts a new upload record.
func (r *Repository) Create(ctx context.Context, upload *Upload) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO uploads (
			id, filename, original_size, compressed_size, file_hash,
			uploaded_at, expires_at, download_count, password_hash,
			deletion_token, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`,
		upload.ID,
		upload.Filename,
		upload.OriginalSize,
		upload.CompressedSize,
		upload.FileHash,
		upload.UploadedAt,
		upload.ExpiresAt,
		upload.DownloadCount,
		upload.PasswordHash,
		upload.DeletionToken,
		upload.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create upload: %w", err)
	}
	return nil
}

// GetByID retrieves an upload by its ID.
func (r *Repository) GetByID(ctx context.Context, id string) (*Upload, error) {
	upload := &Upload{}
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, filename, original_size, compressed_size, file_hash,
			   uploaded_at, expires_at, download_count, password_hash,
			   deletion_token, created_at
		FROM uploads WHERE id = $1
	`, id).Scan(
		&upload.ID,
		&upload.Filename,
		&upload.OriginalSize,
		&upload.CompressedSize,
		&upload.FileHash,
		&upload.UploadedAt,
		&upload.ExpiresAt,
		&upload.DownloadCount,
		&upload.PasswordHash,
		&upload.DeletionToken,
		&upload.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUploadNotFound
		}
		return nil, fmt.Errorf("failed to get upload: %w", err)
	}
	return upload, nil
}

// IncrementDownloadCount atomically increments the download counter.
func (r *Repository) IncrementDownloadCount(ctx context.Context, id string) error {
	tag, err := r.db.Pool.Exec(ctx,
		"UPDATE uploads SET download_count = download_count + 1 WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("failed to increment download count: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrUploadNotFound
	}
	return nil
}

// Delete removes an upload record by ID.
func (r *Repository) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Pool.Exec(ctx, "DELETE FROM uploads WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("failed to delete upload: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrUploadNotFound
	}
	return nil
}

// GetExpired returns all uploads whose expiration time has passed.
func (r *Repository) GetExpired(ctx context.Context) ([]*Upload, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, filename, original_size, compressed_size, file_hash,
			   uploaded_at, expires_at, download_count, password_hash,
			   deletion_token, created_at
		FROM uploads WHERE expires_at < NOW()
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query expired uploads: %w", err)
	}
	defer rows.Close()

	var uploads []*Upload
	for rows.Next() {
		upload := &Upload{}
		if err := rows.Scan(
			&upload.ID,
			&upload.Filename,
			&upload.OriginalSize,
			&upload.CompressedSize,
			&upload.FileHash,
			&upload.UploadedAt,
			&upload.ExpiresAt,
			&upload.DownloadCount,
			&upload.PasswordHash,
			&upload.DeletionToken,
			&upload.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan expired upload: %w", err)
		}
		uploads = append(uploads, upload)
	}
	return uploads, rows.Err()
}

// GetByHash finds uploads with a matching file hash (for deduplication checks).
func (r *Repository) GetByHash(ctx context.Context, hash string) (*Upload, error) {
	upload := &Upload{}
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, filename, original_size, compressed_size, file_hash,
			   uploaded_at, expires_at, download_count, password_hash,
			   deletion_token, created_at
		FROM uploads WHERE file_hash = $1 AND expires_at > NOW()
		LIMIT 1
	`, hash).Scan(
		&upload.ID,
		&upload.Filename,
		&upload.OriginalSize,
		&upload.CompressedSize,
		&upload.FileHash,
		&upload.UploadedAt,
		&upload.ExpiresAt,
		&upload.DownloadCount,
		&upload.PasswordHash,
		&upload.DeletionToken,
		&upload.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // No duplicate found (not an error)
		}
		return nil, fmt.Errorf("failed to query by hash: %w", err)
	}
	return upload, nil
}

// GetStats returns aggregate server statistics.
func (r *Repository) GetStats(ctx context.Context) (*Stats, error) {
	stats := &Stats{}

	err := r.db.Pool.QueryRow(ctx, `
		SELECT
			COUNT(*),
			COUNT(*) FILTER (WHERE expires_at > NOW()),
			COALESCE(SUM(download_count), 0),
			COALESCE(SUM(compressed_size) FILTER (WHERE expires_at > NOW()), 0)
		FROM uploads
	`).Scan(
		&stats.TotalUploads,
		&stats.ActiveUploads,
		&stats.TotalDownloads,
		&stats.StorageUsed,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}
	return stats, nil
}
