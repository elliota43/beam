package service

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"path/filepath"
	"strings"
	"time"

	"beam/internal/server/config"
	"beam/internal/server/database"
	"beam/internal/server/storage"

	"golang.org/x/crypto/bcrypt"
)

// Sentinel errors for the service layer.
var (
	ErrNotFound        = errors.New("upload not found")
	ErrExpired         = errors.New("upload has expired")
	ErrPasswordRequired = errors.New("password required")
	ErrInvalidPassword = errors.New("invalid password")
	ErrInvalidToken    = errors.New("invalid deletion token")
	ErrFileTooLarge    = errors.New("file exceeds maximum allowed size")
	ErrInvalidZip      = errors.New("invalid or corrupt ZIP file")
	ErrDangerousFile   = errors.New("file contains potentially dangerous content")
)

// dangerousExtensions are file extensions that are blocked inside uploaded ZIPs.
var dangerousExtensions = map[string]bool{
	".exe": true, ".bat": true, ".cmd": true, ".com": true,
	".scr": true, ".pif": true, ".vbs": true, ".vbe": true,
	".wsf": true, ".wsh": true, ".msi": true, ".hta": true,
	".lnk": true, ".cpl": true, ".inf": true, ".reg": true,
}

// UploadResult is returned after a successful upload.
type UploadResult struct {
	ID            string    `json:"id"`
	DownloadURL   string    `json:"download_url"`
	DeletionToken string    `json:"deletion_token"`
	ExpiresAt     time.Time `json:"expires_at"`
	Filename      string    `json:"filename"`
	Size          int64     `json:"size"`
}

// UploadInfo is returned for metadata queries.
type UploadInfo struct {
	ID             string    `json:"id"`
	Filename       string    `json:"filename"`
	OriginalSize   int64     `json:"original_size"`
	CompressedSize int64     `json:"compressed_size"`
	UploadedAt     time.Time `json:"uploaded_at"`
	ExpiresAt      time.Time `json:"expires_at"`
	DownloadCount  int       `json:"download_count"`
	HasPassword    bool      `json:"has_password"`
}

// UploadService contains the business logic for file uploads.
type UploadService struct {
	repo  *database.Repository
	store storage.Store
	cfg   *config.Config
}

// NewUploadService creates a new upload service.
func NewUploadService(repo *database.Repository, store storage.Store, cfg *config.Config) *UploadService {
	return &UploadService{
		repo:  repo,
		store: store,
		cfg:   cfg,
	}
}

// ProcessUpload handles an incoming file upload:
// validates the ZIP, calculates its hash, stores it on disk, and creates the DB record.
func (s *UploadService) ProcessUpload(ctx context.Context, filename string, data io.Reader, size int64, password string) (*UploadResult, error) {
	// 1. Check file size limit
	if size > s.cfg.MaxFileSize {
		return nil, ErrFileTooLarge
	}

	// 2. Generate unique ID and deletion token
	uploadID, err := generateSecureToken(16)
	if err != nil {
		return nil, fmt.Errorf("failed to generate upload ID: %w", err)
	}

	deletionToken, err := generateSecureToken(24)
	if err != nil {
		return nil, fmt.Errorf("failed to generate deletion token: %w", err)
	}
	deletionToken = "del_" + deletionToken

	// 3. Read data into buffer while computing SHA-256 hash.
	//    We need the full bytes for ZIP validation anyway.
	hasher := sha256.New()
	tee := io.TeeReader(data, hasher)

	var buf bytes.Buffer
	bytesRead, err := io.Copy(&buf, tee)
	if err != nil {
		return nil, fmt.Errorf("failed to read upload data: %w", err)
	}

	fileHash := hex.EncodeToString(hasher.Sum(nil))
	zipData := buf.Bytes()

	// 4. Validate ZIP magic bytes
	if err := validateZipMagicBytes(zipData); err != nil {
		return nil, err
	}

	// 5. Validate ZIP contents (scan for dangerous extensions)
	originalSize, err := validateAndMeasureZip(zipData)
	if err != nil {
		return nil, err
	}

	// 6. Check for duplicate hash (log only, don't block)
	existing, _ := s.repo.GetByHash(ctx, fileHash)
	if existing != nil {
		slog.Info("duplicate file detected",
			"new_upload", uploadID,
			"existing_upload", existing.ID,
			"hash", fileHash,
		)
	}

	// 7. Store file on disk
	storedBytes, err := s.store.Save(uploadID, bytes.NewReader(zipData))
	if err != nil {
		return nil, fmt.Errorf("failed to store file: %w", err)
	}

	// 8. Hash password if provided
	var passwordHash *string
	if password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			// Clean up stored file
			s.store.Delete(uploadID)
			return nil, fmt.Errorf("failed to hash password: %w", err)
		}
		h := string(hash)
		passwordHash = &h
	}

	// 9. Create database record
	now := time.Now().UTC()
	upload := &database.Upload{
		ID:             uploadID,
		Filename:       sanitizeFilename(filename),
		OriginalSize:   originalSize,
		CompressedSize: storedBytes,
		FileHash:       fileHash,
		UploadedAt:     now,
		ExpiresAt:      now.Add(s.cfg.DefaultExpiry),
		DownloadCount:  0,
		PasswordHash:   passwordHash,
		DeletionToken:  deletionToken,
		CreatedAt:      now,
	}

	if err := s.repo.Create(ctx, upload); err != nil {
		// Clean up stored file on DB failure
		s.store.Delete(uploadID)
		return nil, fmt.Errorf("failed to create upload record: %w", err)
	}

	slog.Info("upload processed",
		"id", uploadID,
		"filename", upload.Filename,
		"original_size", originalSize,
		"compressed_size", bytesRead,
		"hash", fileHash,
	)

	return &UploadResult{
		ID:            uploadID,
		DownloadURL:   fmt.Sprintf("%s/d/%s", s.cfg.BaseURL, uploadID),
		DeletionToken: deletionToken,
		ExpiresAt:     upload.ExpiresAt,
		Filename:      upload.Filename,
		Size:          storedBytes,
	}, nil
}

// GetInfo returns metadata about an upload without serving the file.
func (s *UploadService) GetInfo(ctx context.Context, id string) (*UploadInfo, error) {
	upload, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, database.ErrUploadNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if time.Now().After(upload.ExpiresAt) {
		return nil, ErrExpired
	}

	return &UploadInfo{
		ID:             upload.ID,
		Filename:       upload.Filename,
		OriginalSize:   upload.OriginalSize,
		CompressedSize: upload.CompressedSize,
		UploadedAt:     upload.UploadedAt,
		ExpiresAt:      upload.ExpiresAt,
		DownloadCount:  upload.DownloadCount,
		HasPassword:    upload.PasswordHash != nil,
	}, nil
}

// Download validates the password (if required), increments the download count,
// and returns the path to the file on disk.
func (s *UploadService) Download(ctx context.Context, id string, password string) (filePath string, filename string, err error) {
	upload, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, database.ErrUploadNotFound) {
			return "", "", ErrNotFound
		}
		return "", "", err
	}

	if time.Now().After(upload.ExpiresAt) {
		return "", "", ErrExpired
	}

	// Check password if the upload is password-protected
	if upload.PasswordHash != nil {
		if password == "" {
			return "", "", ErrPasswordRequired
		}
		if err := bcrypt.CompareHashAndPassword([]byte(*upload.PasswordHash), []byte(password)); err != nil {
			return "", "", ErrInvalidPassword
		}
	}

	// Get the file path from storage
	path, err := s.store.GetPath(id)
	if err != nil {
		return "", "", fmt.Errorf("file not found on disk: %w", err)
	}

	// Increment download count (best-effort, don't fail the download)
	if err := s.repo.IncrementDownloadCount(ctx, id); err != nil {
		slog.Error("failed to increment download count", "id", id, "error", err)
	}

	return path, upload.Filename, nil
}

// DeleteUpload validates the deletion token and removes the upload.
func (s *UploadService) DeleteUpload(ctx context.Context, id string, token string) error {
	upload, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, database.ErrUploadNotFound) {
			return ErrNotFound
		}
		return err
	}

	if upload.DeletionToken != token {
		return ErrInvalidToken
	}

	// Delete file from storage
	if err := s.store.Delete(id); err != nil {
		slog.Error("failed to delete file from storage", "id", id, "error", err)
		// Continue with DB deletion even if file deletion fails
	}

	// Delete record from database
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete upload record: %w", err)
	}

	slog.Info("upload deleted", "id", id, "filename", upload.Filename)
	return nil
}

// GetStats returns aggregate server statistics.
func (s *UploadService) GetStats(ctx context.Context) (*database.Stats, error) {
	return s.repo.GetStats(ctx)
}

// --- Helpers ---

// generateSecureToken produces a cryptographically secure, URL-safe random string.
func generateSecureToken(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", fmt.Errorf("crypto/rand failure: %w", err)
		}
		result[i] = charset[n.Int64()]
	}
	return string(result), nil
}

// validateZipMagicBytes checks that data starts with the ZIP magic number (PK\x03\x04).
func validateZipMagicBytes(data []byte) error {
	if len(data) < 4 {
		return ErrInvalidZip
	}
	// Standard ZIP local file header: PK\x03\x04
	// Empty ZIP (end of central directory): PK\x05\x06
	if data[0] == 0x50 && data[1] == 0x4B {
		if (data[2] == 0x03 && data[3] == 0x04) || // local file header
			(data[2] == 0x05 && data[3] == 0x06) { // empty archive
			return nil
		}
	}
	return ErrInvalidZip
}

// validateAndMeasureZip opens the ZIP, scans for dangerous extensions,
// and returns the total uncompressed size of all entries.
func validateAndMeasureZip(data []byte) (int64, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrInvalidZip, err)
	}

	var totalUncompressed int64
	for _, f := range reader.File {
		ext := strings.ToLower(filepath.Ext(f.Name))
		if dangerousExtensions[ext] {
			return 0, fmt.Errorf("%w: blocked extension %s in %s", ErrDangerousFile, ext, f.Name)
		}
		totalUncompressed += int64(f.UncompressedSize64)
	}

	return totalUncompressed, nil
}

// sanitizeFilename strips directory components and limits length.
func sanitizeFilename(name string) string {
	// Normalize Windows-style backslashes to forward slashes before
	// calling filepath.Base, which is platform-specific.
	name = strings.ReplaceAll(name, "\\", "/")

	// Take only the base name
	name = filepath.Base(name)

	// Limit length
	if len(name) > 255 {
		ext := filepath.Ext(name)
		name = name[:255-len(ext)] + ext
	}

	if name == "" || name == "." {
		name = "upload.zip"
	}

	return name
}
