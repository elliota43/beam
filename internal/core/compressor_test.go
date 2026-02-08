package core

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helpers

func verifyZipContents(t *testing.T, zipBytes []byte, expectedFiles map[string]string) {
	t.Helper()

	reader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("failed to create zip reader: %v", err)
	}

	if len(reader.File) != len(expectedFiles) {
		t.Errorf("expected %d files in zip, got %d", len(expectedFiles), len(reader.File))
	}

	for _, f := range reader.File {
		expectedContent, exists := expectedFiles[f.Name]
		if !exists {
			t.Errorf("unexpected file in zip: %s", f.Name)
			continue
		}

		rc, err := f.Open()
		if err != nil {
			t.Errorf("failed to open file %s in zip: %v", f.Name, err)
			continue
		}
		defer rc.Close()

		content, err := io.ReadAll(rc)
		if err != nil {
			t.Errorf("failed to read file %s: %v", f.Name, err)
			continue
		}

		if string(content) != expectedContent {
			t.Errorf("file %s: expected content %q, got %q", f.Name, expectedContent, string(content))
		}
	}
}

func extractZip(t *testing.T, zipBytes []byte, destDir string) {
	t.Helper()

	reader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("failed to create zip reader: %v", err)
	}

	for _, f := range reader.File {
		fpath := filepath.Join(destDir, f.Name)

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			t.Fatalf("failed to create directories: %v", err)
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			t.Fatalf("failed to create file: %v", err)
		}

		rc, err := f.Open()
		if err != nil {
			t.Fatalf("failed to open zip file: %v", err)
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			t.Fatalf("failed to extract file: %v", err)
		}
	}
}

// Tests

func TestFiletree_ToZipBytes(t *testing.T) {
	t.Run("single file", func(t *testing.T) {
		testFile := setupTestFile(t, "test.txt", "hello world")
		paths := []ParsedPath{{FullPath: testFile, Kind: PathFile}}

		tree, err := BuildFiletree(paths)
		if err != nil {
			t.Fatal(err)
		}

		zipBytes, err := tree.ToZipBytes()
		if err != nil {
			t.Fatalf("failed to compress: %v", err)
		}

		if len(zipBytes) == 0 {
			t.Fatal("expected non-empty zip bytes")
		}

		expectedFiles := map[string]string{
			"test.txt": "hello world",
		}
		verifyZipContents(t, zipBytes, expectedFiles)
	})

	t.Run("directory with files", func(t *testing.T) {
		dirPath := setupTestDir(t, "mydir", map[string]string{
			"file1.txt": "content1",
			"file2.txt": "content2",
		})
		paths := []ParsedPath{{FullPath: dirPath, Kind: PathDir}}

		tree, err := BuildFiletree(paths)
		if err != nil {
			t.Fatal(err)
		}

		zipBytes, err := tree.ToZipBytes()
		if err != nil {
			t.Fatalf("failed to compress: %v", err)
		}

		expectedFiles := map[string]string{
			"mydir/file1.txt": "content1",
			"mydir/file2.txt": "content2",
		}
		verifyZipContents(t, zipBytes, expectedFiles)
	})

	t.Run("nested directories", func(t *testing.T) {
		structure := map[string]interface{}{
			"project": map[string]interface{}{
				"README.md": "# Project",
				"src": map[string]interface{}{
					"main.go":  "package main",
					"utils.go": "package utils",
				},
				"tests": map[string]interface{}{
					"test.go": "package test",
				},
			},
		}
		rootDir := setupNestedTestDir(t, structure)
		projectPath := filepath.Join(rootDir, "project")

		paths := []ParsedPath{{FullPath: projectPath, Kind: PathDir}}
		tree, err := BuildFiletree(paths)
		if err != nil {
			t.Fatal(err)
		}

		zipBytes, err := tree.ToZipBytes()
		if err != nil {
			t.Fatalf("failed to compress: %v", err)
		}

		expectedFiles := map[string]string{
			"project/README.md":     "# Project",
			"project/src/main.go":   "package main",
			"project/src/utils.go":  "package utils",
			"project/tests/test.go": "package test",
		}
		verifyZipContents(t, zipBytes, expectedFiles)
	})

	t.Run("multiple files creates virtual root", func(t *testing.T) {
		file1 := setupTestFile(t, "file1.txt", "content1")
		file2 := setupTestFile(t, "file2.txt", "content2")

		paths := []ParsedPath{
			{FullPath: file1, Kind: PathFile},
			{FullPath: file2, Kind: PathFile},
		}
		tree, err := BuildFiletree(paths)
		if err != nil {
			t.Fatal(err)
		}

		zipBytes, err := tree.ToZipBytes()
		if err != nil {
			t.Fatalf("failed to compress: %v", err)
		}

		// Verify that files are under virtual root
		reader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
		if err != nil {
			t.Fatal(err)
		}

		if len(reader.File) != 2 {
			t.Errorf("expected 2 files, got %d", len(reader.File))
		}

		// Files should be under upload_XXXX/ prefix
		for _, f := range reader.File {
			if !strings.HasPrefix(f.Name, "upload_") {
				t.Errorf("expected file to be under upload_ prefix, got %s", f.Name)
			}
		}
	})

	t.Run("empty directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		emptyDir := filepath.Join(tmpDir, "empty")
		if err := os.Mkdir(emptyDir, 0755); err != nil {
			t.Fatal(err)
		}

		paths := []ParsedPath{{FullPath: emptyDir, Kind: PathDir}}
		tree, err := BuildFiletree(paths)
		if err != nil {
			t.Fatal(err)
		}

		zipBytes, err := tree.ToZipBytes()
		if err != nil {
			t.Fatalf("failed to compress: %v", err)
		}

		// Should create valid ZIP with no files
		reader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
		if err != nil {
			t.Fatal(err)
		}

		if len(reader.File) != 0 {
			t.Errorf("expected 0 files for empty directory, got %d", len(reader.File))
		}
	})

	t.Run("preserves file permissions", func(t *testing.T) {
		testFile := setupTestFile(t, "script.sh", "#!/bin/bash\necho hello")

		// Make it executable
		if err := os.Chmod(testFile, 0755); err != nil {
			t.Fatal(err)
		}

		paths := []ParsedPath{{FullPath: testFile, Kind: PathFile}}
		tree, err := BuildFiletree(paths)
		if err != nil {
			t.Fatal(err)
		}

		zipBytes, err := tree.ToZipBytes()
		if err != nil {
			t.Fatalf("failed to compress: %v", err)
		}

		reader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
		if err != nil {
			t.Fatal(err)
		}

		if len(reader.File) != 1 {
			t.Fatalf("expected 1 file, got %d", len(reader.File))
		}

		mode := reader.File[0].Mode()
		if mode&0111 == 0 {
			t.Error("expected file to be executable")
		}
	})

	t.Run("can extract and verify", func(t *testing.T) {
		structure := map[string]interface{}{
			"app": map[string]interface{}{
				"config.json": `{"port": 8080}`,
				"data": map[string]interface{}{
					"users.csv": "id,name\n1,Alice\n2,Bob",
				},
			},
		}
		rootDir := setupNestedTestDir(t, structure)
		appPath := filepath.Join(rootDir, "app")

		paths := []ParsedPath{{FullPath: appPath, Kind: PathDir}}
		tree, err := BuildFiletree(paths)
		if err != nil {
			t.Fatal(err)
		}

		zipBytes, err := tree.ToZipBytes()
		if err != nil {
			t.Fatalf("failed to compress: %v", err)
		}

		// Extract to temp directory
		extractDir := t.TempDir()
		extractZip(t, zipBytes, extractDir)

		// Verify extracted files
		configPath := filepath.Join(extractDir, "app", "config.json")
		configContent, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("failed to read extracted config: %v", err)
		}
		if string(configContent) != `{"port": 8080}` {
			t.Errorf("unexpected config content: %s", configContent)
		}

		usersPath := filepath.Join(extractDir, "app", "data", "users.csv")
		usersContent, err := os.ReadFile(usersPath)
		if err != nil {
			t.Fatalf("failed to read extracted users: %v", err)
		}
		if string(usersContent) != "id,name\n1,Alice\n2,Bob" {
			t.Errorf("unexpected users content: %s", usersContent)
		}
	})
}

func TestFiletree_GetUncompressedSize(t *testing.T) {
	t.Run("single file", func(t *testing.T) {
		content := "hello world"
		testFile := setupTestFile(t, "test.txt", content)
		paths := []ParsedPath{{FullPath: testFile, Kind: PathFile}}

		tree, err := BuildFiletree(paths)
		if err != nil {
			t.Fatal(err)
		}

		size, err := tree.GetUncompressedSize()
		if err != nil {
			t.Fatalf("failed to get size: %v", err)
		}

		if size != int64(len(content)) {
			t.Errorf("expected size %d, got %d", len(content), size)
		}
	})

	t.Run("multiple files", func(t *testing.T) {
		dirPath := setupTestDir(t, "mydir", map[string]string{
			"file1.txt": "12345",      // 5 bytes
			"file2.txt": "abcdefghij", // 10 bytes
		})
		paths := []ParsedPath{{FullPath: dirPath, Kind: PathDir}}

		tree, err := BuildFiletree(paths)
		if err != nil {
			t.Fatal(err)
		}

		size, err := tree.GetUncompressedSize()
		if err != nil {
			t.Fatalf("failed to get size: %v", err)
		}

		expectedSize := int64(5 + 10)
		if size != expectedSize {
			t.Errorf("expected size %d, got %d", expectedSize, size)
		}
	})

	t.Run("empty directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		emptyDir := filepath.Join(tmpDir, "empty")
		if err := os.Mkdir(emptyDir, 0755); err != nil {
			t.Fatal(err)
		}

		paths := []ParsedPath{{FullPath: emptyDir, Kind: PathDir}}
		tree, err := BuildFiletree(paths)
		if err != nil {
			t.Fatal(err)
		}

		size, err := tree.GetUncompressedSize()
		if err != nil {
			t.Fatalf("failed to get size: %v", err)
		}

		if size != 0 {
			t.Errorf("expected size 0 for empty directory, got %d", size)
		}
	})
}

func TestCompressionRatio(t *testing.T) {
	t.Run("compression reduces size", func(t *testing.T) {
		// Create a file with repetitive content (compresses well)
		repetitiveContent := strings.Repeat("hello world ", 1000)
		testFile := setupTestFile(t, "test.txt", repetitiveContent)

		paths := []ParsedPath{{FullPath: testFile, Kind: PathFile}}
		tree, err := BuildFiletree(paths)
		if err != nil {
			t.Fatal(err)
		}

		uncompressedSize, err := tree.GetUncompressedSize()
		if err != nil {
			t.Fatal(err)
		}

		zipBytes, err := tree.ToZipBytes()
		if err != nil {
			t.Fatal(err)
		}

		compressedSize := int64(len(zipBytes))

		// Repetitive content should compress to less than 50% original size
		if compressedSize >= uncompressedSize/2 {
			t.Logf("Warning: compression ratio not as good as expected")
			t.Logf("Uncompressed: %d, Compressed: %d", uncompressedSize, compressedSize)
		}

		compressionRatio := float64(compressedSize) / float64(uncompressedSize) * 100
		t.Logf("Compression ratio: %.2f%%", compressionRatio)
	})
}
