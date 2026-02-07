package core

import (
	"os"
	"path/filepath"
	"testing"
)

// Helpers

func setupTestFile(t *testing.T, name string, content string) string {
	t.Helper()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, name)
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	return filePath
}

func setupTestDir(t *testing.T, name string, files map[string]string) string {
	t.Helper()
	tmpDir := t.TempDir()
	dirPath := filepath.Join(tmpDir, name)
	if err := os.Mkdir(dirPath, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	for filename, content := range files {
		filePath := filepath.Join(dirPath, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create file %s: %v", filename, err)
		}
	}
	return dirPath
}

func setupNestedTestDir(t *testing.T, structure map[string]interface{}) string {
	t.Helper()
	rootDir := t.TempDir()
	createStructure(t, rootDir, structure)
	return rootDir
}

func createStructure(t *testing.T, basePath string, structure map[string]interface{}) {
	t.Helper()
	for name, content := range structure {
		path := filepath.Join(basePath, name)

		switch v := content.(type) {
		case string:
			// file
			if err := os.WriteFile(path, []byte(v), 0644); err != nil {
				t.Fatalf("failed to create file %s: %v", path, err)
			}

		case map[string]interface{}:
			// dir
			if err := os.Mkdir(path, 0755); err != nil {
				t.Fatalf("failed to create directory %s: %v", path, err)
			}
			createStructure(t, path, v)
		default:
			t.Fatalf("unsupported structure type for %s", name)
		}
	}
}

func assertNodeType(t *testing.T, node Node, expectedType string) {
	t.Helper()
	switch expectedType {
	case "file":
		if _, ok := node.(*File); !ok {
			t.Errorf("expected node to be a file, got %T", node)
		}
	case "dir":
		if _, ok := node.(*Dir); !ok {
			t.Errorf("expected node to be a dir, got %T", node)
		}
	default:
		t.Fatalf("unknown expected type: %s", expectedType)
	}
}

func assertDirChildCount(t *testing.T, dir *Dir, expected int) {
	t.Helper()
	if len(dir.children) != expected {
		t.Errorf("expected %d children, got %d", expected, len(dir.children))
	}
}

// Tests

func TestBuildFiletree(t *testing.T) {
	t.Run("single file", func(t *testing.T) {
		testFile := setupTestFile(t, "test.txt", "content")
		paths := []ParsedPath{{FullPath: testFile, Kind: PathFile}}

		tree, err := BuildFiletree(paths)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if tree.Root == nil {
			t.Fatalf("expected root to be non-nil")
		}

		root := *tree.Root
		assertNodeType(t, root, "file")

		file := root.(*File)
		if file.Name() != "test.txt" {
			t.Errorf("expected name 'test.txt', got %s", file.Name())
		}
	})

	t.Run("single directory", func(t *testing.T) {
		dirPath := setupTestDir(t, "subdir", map[string]string{
			"file1.txt": "content1",
			"file2.txt": "content2",
		})
		paths := []ParsedPath{{FullPath: dirPath, Kind: PathDir}}

		tree, err := BuildFiletree(paths)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		root := *tree.Root
		assertNodeType(t, root, "dir")

		dir := root.(*Dir)
		if dir.Name() != "subdir" {
			t.Errorf("expected name 'subdir', got %s", dir.Name())
		}
		assertDirChildCount(t, dir, 2)
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
			t.Fatalf("expected no error, got %v", err)
		}

		root := *tree.Root
		assertNodeType(t, root, "dir")

		dir := root.(*Dir)
		assertDirChildCount(t, dir, 2)

		if len(dir.Name()) < 7 || dir.Name()[:7] != "upload_" {
			t.Errorf("expected virtual root name to start with 'upload_', got %s", dir.Name())
		}
	})

	t.Run("nested directories", func(t *testing.T) {
		structure := map[string]interface{}{
			"level1": map[string]interface{}{
				"level2": map[string]interface{}{
					"deep.txt": "deep content",
				},
			},
		}
		rootDir := setupNestedTestDir(t, structure)
		level1Path := filepath.Join(rootDir, "level1")

		paths := []ParsedPath{{FullPath: level1Path, Kind: PathDir}}
		tree, err := BuildFiletree(paths)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		root := *tree.Root
		level1 := root.(*Dir)
		assertDirChildCount(t, level1, 1)

		level2 := level1.children[0].(*Dir)
		assertDirChildCount(t, level2, 1)

		file := level2.children[0].(*File)
		if file.Name() != "deep.txt" {
			t.Errorf("expected file name 'deep.txt', got %s", file.Name())
		}
	})

	t.Run("empty paths returns error", func(t *testing.T) {
		paths := []ParsedPath{}
		tree, err := BuildFiletree(paths)

		if err == nil {
			t.Fatal("expected error for empty paths")
		}
		if tree != nil {
			t.Error("expected nil tree for empty paths")
		}
	})

	t.Run("parent child links are correct", func(t *testing.T) {
		dirPath := setupTestDir(t, "parent", map[string]string{
			"child.txt": "content",
		})

		paths := []ParsedPath{{FullPath: dirPath, Kind: PathDir}}
		tree, err := BuildFiletree(paths)
		if err != nil {
			t.Fatal(err)
		}

		root := *tree.Root
		parentDir := root.(*Dir)
		childFile := parentDir.children[0].(*File)

		if childFile.dir != parentDir {
			t.Error("expected child file's dir to point to parent")
		}
	})

	t.Run("complex nested structure", func(t *testing.T) {
		structure := map[string]interface{}{
			"project": map[string]interface{}{
				"src": map[string]interface{}{
					"main.go":  "package main",
					"utils.go": "package main",
				},
				"tests": map[string]interface{}{
					"main_test.go": "package main",
				},
				"README.md": "# Project",
			},
		}

		rootDir := setupNestedTestDir(t, structure)
		projectPath := filepath.Join(rootDir, "project")

		paths := []ParsedPath{{FullPath: projectPath, Kind: PathDir}}
		tree, err := BuildFiletree(paths)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		root := *tree.Root
		project := root.(*Dir)

		// should have 3 children: src/, tests/, Readme.md
		assertDirChildCount(t, project, 3)
	})
}

func TestCreateVirtualRoot(t *testing.T) {
	file1 := &File{path: "/tmp/file1.txt", name: "file1.txt"}
	file2 := &File{path: "/tmp/file2.txt", name: "file2.txt"}
	children := []Node{file1, file2}

	virtualRoot := createVirtualRoot(children)

	assertDirChildCount(t, virtualRoot, 2)

	if file1.dir != virtualRoot {
		t.Error("expected file1 dir to be set to virtualRoot")
	}
	if file2.dir != virtualRoot {
		t.Error("expected file2 dir to be sert to virtualRoot")
	}
	if virtualRoot.parent != nil {
		t.Error("expected virtual root to have no parent")
	}
}
