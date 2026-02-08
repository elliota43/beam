package storage

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileSystemStore_Save(t *testing.T) {
	t.Run("saves file to disk", func(t *testing.T) {
		dir := t.TempDir()
		store := NewFileSystemStore(dir)

		data := bytes.NewReader([]byte("test content"))
		n, err := store.Save("abc123", data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if n != 12 {
			t.Errorf("expected 12 bytes written, got %d", n)
		}

		// Verify file exists on disk
		content, err := os.ReadFile(filepath.Join(dir, "abc123.zip"))
		if err != nil {
			t.Fatalf("failed to read saved file: %v", err)
		}
		if string(content) != "test content" {
			t.Errorf("expected 'test content', got %q", content)
		}
	})

	t.Run("saves large content", func(t *testing.T) {
		dir := t.TempDir()
		store := NewFileSystemStore(dir)

		largeContent := strings.Repeat("x", 1024*1024) // 1MB
		data := bytes.NewReader([]byte(largeContent))
		n, err := store.Save("large", data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if n != int64(len(largeContent)) {
			t.Errorf("expected %d bytes, got %d", len(largeContent), n)
		}
	})
}

func TestFileSystemStore_GetPath(t *testing.T) {
	t.Run("returns path for existing file", func(t *testing.T) {
		dir := t.TempDir()
		store := NewFileSystemStore(dir)

		// Create the file first
		filePath := filepath.Join(dir, "test123.zip")
		os.WriteFile(filePath, []byte("data"), 0644)

		path, err := store.GetPath("test123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if path != filePath {
			t.Errorf("expected %s, got %s", filePath, path)
		}
	})

	t.Run("returns error for missing file", func(t *testing.T) {
		dir := t.TempDir()
		store := NewFileSystemStore(dir)

		_, err := store.GetPath("nonexistent")
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})
}

func TestFileSystemStore_Delete(t *testing.T) {
	t.Run("deletes existing file", func(t *testing.T) {
		dir := t.TempDir()
		store := NewFileSystemStore(dir)

		// Create the file
		filePath := filepath.Join(dir, "del123.zip")
		os.WriteFile(filePath, []byte("data"), 0644)

		if err := store.Delete("del123"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify file is gone
		if _, err := os.Stat(filePath); !os.IsNotExist(err) {
			t.Error("expected file to be deleted")
		}
	})

	t.Run("no error for missing file", func(t *testing.T) {
		dir := t.TempDir()
		store := NewFileSystemStore(dir)

		if err := store.Delete("nonexistent"); err != nil {
			t.Errorf("expected no error for missing file, got: %v", err)
		}
	})
}

func TestFileSystemStore_EnsureDir(t *testing.T) {
	t.Run("creates directory", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "nested", "storage", "path")
		store := NewFileSystemStore(dir)

		if err := store.EnsureDir(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("directory not created: %v", err)
		}
		if !info.IsDir() {
			t.Error("expected a directory")
		}
	})

	t.Run("succeeds if directory exists", func(t *testing.T) {
		dir := t.TempDir()
		store := NewFileSystemStore(dir)

		if err := store.EnsureDir(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
