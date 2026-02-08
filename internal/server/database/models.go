package database

import "time"

// Upload represents a stored file upload in the database.
type Upload struct {
	ID             string
	Filename       string
	OriginalSize   int64
	CompressedSize int64
	FileHash       string
	UploadedAt     time.Time
	ExpiresAt      time.Time
	DownloadCount  int
	PasswordHash   *string // nil when no password set
	DeletionToken  string
	CreatedAt      time.Time
}

// Stats holds aggregate server statistics.
type Stats struct {
	TotalUploads   int64
	ActiveUploads  int64
	TotalDownloads int64
	StorageUsed    int64
}
