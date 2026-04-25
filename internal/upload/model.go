package upload

import "time"

type Upload struct {
	ID        string
	Slug      string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type UploadedFile struct {
	ID           string
	UploadID     string
	OriginalName string
	StoredName   string
	Size         int64
	ContentType  string
	SHA256       string
	CreatedAt    time.Time
}
