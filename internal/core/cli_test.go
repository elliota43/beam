package core

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestFiles(t *testing.T, files map[string]string) []string {
	t.Helper()
	tmpDir := t.TempDir()
	var paths []string

	for filename, content := range files {
		filePath := filepath.Join(tmpDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create test file %s: %v", filename, err)
		}
		paths = append(paths, filePath)
	}

	return paths
}

func assertParsedPath(t *testing.T, parsed ParsedPath, expectedPath string, expectedKind PathKind) {
	t.Helper()
	if parsed.FullPath != expectedPath {
		t.Errorf("expected path %s, got %s", expectedPath, parsed.FullPath)
	}
	if parsed.Kind != expectedKind {
		t.Errorf("expected kind %v, got %v", expectedKind, parsed.Kind)
	}
}

func assertValidationError(t *testing.T, err error, expectedArg string, expectedCause string) {
	t.Helper()
	validationErr, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if expectedArg != "" && validationErr.Arg != expectedArg {
		t.Errorf("expected Arg to be %q, got %q", expectedArg, validationErr.Arg)
	}
	if expectedCause != "" && validationErr.Cause != expectedCause {
		t.Errorf("expected Cause to be %q, got %q", expectedCause, validationErr.Cause)
	}
}

// Tests

func TestParseArgs(t *testing.T) {
	t.Run("empty args returns error", func(t *testing.T) {
		result, err := ParseArgs([]string{})

		if err == nil {
			t.Fatal("expected error for empty args")
		}
		if result != nil {
			t.Error("expected nil result for empty args")
		}
		assertValidationError(t, err, "<files>", "no files provided")
	})

	t.Run("single file", func(t *testing.T) {
		paths := setupTestFiles(t, map[string]string{
			"test.txt": "content",
		})

		result, err := ParseArgs(paths)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(result) != 1 {
			t.Fatalf("expected 1 result, got %d", len(result))
		}
		assertParsedPath(t, result[0], paths[0], PathFile)
	})

	t.Run("single directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		result, err := ParseArgs([]string{tmpDir})

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(result) != 1 {
			t.Fatalf("expected 1 result, got %d", len(result))
		}
		assertParsedPath(t, result[0], tmpDir, PathDir)
	})

	t.Run("multiple files", func(t *testing.T) {
		paths := setupTestFiles(t, map[string]string{
			"file1.txt": "content1",
			"file2.txt": "content2",
		})

		result, err := ParseArgs(paths)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(result) != 2 {
			t.Fatalf("expected 2 results, got %d", len(result))
		}
		for _, r := range result {
			if r.Kind != PathFile {
				t.Error("expected PathFile kind")
			}
		}
	})

	t.Run("nonexistent path returns error", func(t *testing.T) {
		result, err := ParseArgs([]string{"/nonexistent/path/file.txt"})

		if err == nil {
			t.Fatal("expected error for nonexistent path")
		}
		if result != nil {
			t.Error("expected nil result for nonexistent path")
		}
		assertValidationError(t, err, "", "not found or not accessible")
	})

	t.Run("path cleaning", func(t *testing.T) {
		paths := setupTestFiles(t, map[string]string{
			"test.txt": "content",
		})
		testFile := paths[0]
		tmpDir := filepath.Dir(testFile)

		messyPath := filepath.Join(tmpDir, ".", "test.txt")
		result, err := ParseArgs([]string{messyPath})

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		assertParsedPath(t, result[0], testFile, PathFile)
	})

	t.Run("mixed files and directories", func(t *testing.T) {
		tmpDir := t.TempDir()
		subDir := filepath.Join(tmpDir, "subdir")
		if err := os.Mkdir(subDir, 0755); err != nil {
			t.Fatal(err)
		}
		testFile := filepath.Join(tmpDir, "test.txt")
		if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := ParseArgs([]string{testFile, subDir})

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(result) != 2 {
			t.Fatalf("expected 2 results, got %d", len(result))
		}
		assertParsedPath(t, result[0], testFile, PathFile)
		assertParsedPath(t, result[1], subDir, PathDir)
	})
}

func TestValidationError(t *testing.T) {
	t.Run("error message format", func(t *testing.T) {
		err := &ValidationError{
			Arg:   "test.txt",
			Cause: "file not found",
		}

		expected := `invalid argument "test.txt": file not found`
		if err.Error() != expected {
			t.Errorf("expected error message %q, got %q", expected, err.Error())
		}
	})
}
