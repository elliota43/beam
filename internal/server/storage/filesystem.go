package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Store defines the interface for file storage backends.
// This allows swapping filesystem for S3 or other backends later.
type Store interface {
	Save(uploadID string, data io.Reader) (int64, error)
	GetPath(uploadID string) (string, error)
	Delete(uploadID string) error
	EnsureDir() error
}

// FileSystemStore stores uploaded files on the local filesystem.
type FileSystemStore struct {
	basePath string
}

// NewFileSystemStore creates a new filesystem storage backend.
func NewFileSystemStore(basePath string) *FileSystemStore {
	return &FileSystemStore{basePath: basePath}
}

// EnsureDir creates the storage directory if it doesn't exist.
func (fs *FileSystemStore) EnsureDir() error {
	if err := os.MkdirAll(fs.basePath, 0755); err != nil {
		return fmt.Errorf("failed to create storage directory %s: %w", fs.basePath, err)
	}
	return nil
}

// Save writes data from a reader to a file named {uploadID}.zip.
// Returns the number of bytes written.
func (fs *FileSystemStore) Save(uploadID string, data io.Reader) (int64, error) {
	filePath := fs.filePath(uploadID)

	file, err := os.Create(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to create file %s: %w", filePath, err)
	}
	defer file.Close()

	n, err := io.Copy(file, data)
	if err != nil {
		// Clean up partial file on error
		os.Remove(filePath)
		return 0, fmt.Errorf("failed to write file: %w", err)
	}

	return n, nil
}

// GetPath returns the absolute path to a stored upload file.
// Returns an error if the file does not exist.
func (fs *FileSystemStore) GetPath(uploadID string) (string, error) {
	filePath := fs.filePath(uploadID)

	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file not found for upload %s", uploadID)
		}
		return "", fmt.Errorf("failed to stat file: %w", err)
	}

	return filePath, nil
}

// Delete removes the stored file for an upload.
func (fs *FileSystemStore) Delete(uploadID string) error {
	filePath := fs.filePath(uploadID)
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file %s: %w", filePath, err)
	}
	return nil
}

func (fs *FileSystemStore) filePath(uploadID string) string {
	return filepath.Join(fs.basePath, uploadID+".zip")
}
