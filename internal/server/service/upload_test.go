package service

import (
	"archive/zip"
	"bytes"
	"testing"
)

// --- Helper to create valid ZIP bytes for testing ---

func createTestZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range files {
		f, err := w.Create(name)
		if err != nil {
			t.Fatalf("failed to create zip entry %s: %v", name, err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			t.Fatalf("failed to write zip entry %s: %v", name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("failed to close zip writer: %v", err)
	}
	return buf.Bytes()
}

// --- Token generation ---

func TestGenerateSecureToken(t *testing.T) {
	t.Run("generates correct length", func(t *testing.T) {
		for _, length := range []int{8, 16, 24, 32} {
			token, err := generateSecureToken(length)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(token) != length {
				t.Errorf("expected length %d, got %d", length, len(token))
			}
		}
	})

	t.Run("generates unique tokens", func(t *testing.T) {
		seen := make(map[string]bool)
		for i := 0; i < 100; i++ {
			token, err := generateSecureToken(16)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if seen[token] {
				t.Fatalf("duplicate token generated: %s", token)
			}
			seen[token] = true
		}
	})

	t.Run("only contains URL-safe characters", func(t *testing.T) {
		token, err := generateSecureToken(100)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
		for _, c := range token {
			found := false
			for _, valid := range charset {
				if c == valid {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("token contains invalid character: %c", c)
			}
		}
	})
}

// --- ZIP validation ---

func TestValidateZipMagicBytes(t *testing.T) {
	t.Run("valid ZIP bytes", func(t *testing.T) {
		zipData := createTestZip(t, map[string]string{"test.txt": "hello"})
		if err := validateZipMagicBytes(zipData); err != nil {
			t.Errorf("expected valid ZIP, got error: %v", err)
		}
	})

	t.Run("empty data", func(t *testing.T) {
		if err := validateZipMagicBytes([]byte{}); err == nil {
			t.Error("expected error for empty data")
		}
	})

	t.Run("too short", func(t *testing.T) {
		if err := validateZipMagicBytes([]byte{0x50, 0x4B}); err == nil {
			t.Error("expected error for data shorter than 4 bytes")
		}
	})

	t.Run("invalid magic bytes", func(t *testing.T) {
		if err := validateZipMagicBytes([]byte{0x00, 0x01, 0x02, 0x03}); err == nil {
			t.Error("expected error for non-ZIP data")
		}
	})

	t.Run("PDF magic bytes rejected", func(t *testing.T) {
		if err := validateZipMagicBytes([]byte("%PDF")); err == nil {
			t.Error("expected error for PDF data")
		}
	})
}

func TestValidateAndMeasureZip(t *testing.T) {
	t.Run("valid ZIP with safe files", func(t *testing.T) {
		zipData := createTestZip(t, map[string]string{
			"readme.txt":  "hello world",
			"data.json":   `{"key": "value"}`,
			"src/main.go": "package main",
		})

		size, err := validateAndMeasureZip(zipData)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expectedSize := int64(len("hello world") + len(`{"key": "value"}`) + len("package main"))
		if size != expectedSize {
			t.Errorf("expected size %d, got %d", expectedSize, size)
		}
	})

	t.Run("rejects ZIP with .exe file", func(t *testing.T) {
		zipData := createTestZip(t, map[string]string{
			"readme.txt":  "hello",
			"malware.exe": "MZ...",
		})

		_, err := validateAndMeasureZip(zipData)
		if err == nil {
			t.Error("expected error for ZIP containing .exe")
		}
	})

	t.Run("rejects ZIP with .bat file", func(t *testing.T) {
		zipData := createTestZip(t, map[string]string{
			"script.bat": "echo hello",
		})

		_, err := validateAndMeasureZip(zipData)
		if err == nil {
			t.Error("expected error for ZIP containing .bat")
		}
	})

	t.Run("rejects ZIP with .vbs file", func(t *testing.T) {
		zipData := createTestZip(t, map[string]string{
			"script.vbs": "MsgBox Hello",
		})

		_, err := validateAndMeasureZip(zipData)
		if err == nil {
			t.Error("expected error for ZIP containing .vbs")
		}
	})

	t.Run("allows common developer file types", func(t *testing.T) {
		zipData := createTestZip(t, map[string]string{
			"app.js":    "console.log('hi')",
			"style.css": "body {}",
			"index.html": "<html></html>",
			"lib.py":    "print('hi')",
			"Makefile":  "all: build",
		})

		_, err := validateAndMeasureZip(zipData)
		if err != nil {
			t.Errorf("common dev files should be allowed, got error: %v", err)
		}
	})

	t.Run("invalid ZIP data", func(t *testing.T) {
		_, err := validateAndMeasureZip([]byte("not a zip file"))
		if err == nil {
			t.Error("expected error for invalid ZIP data")
		}
	})
}

// --- Filename sanitization ---

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple name", "file.zip", "file.zip"},
		{"strips directory", "/path/to/file.zip", "file.zip"},
		{"strips windows path", "C:\\Users\\test\\file.zip", "file.zip"},
		{"empty name", "", "upload.zip"},
		{"dot name", ".", "upload.zip"},
		{"replaces slashes", "a/b/c.zip", "c.zip"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeFilename(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
