package core

import "testing"

// Test helpers

func newTestFile(path, name string) *File {
	return &File{
		path: path,
		name: name,
	}
}

func newTestDir(path, name string) *Dir {
	return &Dir{
		path:     path,
		name:     name,
		children: []Node{},
	}
}

// Tests

func TestFile(t *testing.T) {
	t.Run("Path returns correct path", func(t *testing.T) {
		file := newTestFile("/home/user/document.txt", "document.txt")

		if file.Path() != "/home/user/document.txt" {
			t.Errorf("expected '/home/user/document.txt', got %s", file.Path())
		}
	})

	t.Run("Name returns correct name", func(t *testing.T) {
		file := newTestFile("/home/user/document.txt", "document.txt")

		if file.Name() != "document.txt" {
			t.Errorf("expected 'document.txt', got %s", file.Name())
		}
	})
}

func TestDir(t *testing.T) {
	t.Run("Path returns correct path", func(t *testing.T) {
		dir := newTestDir("/home/user/documents", "documents")

		if dir.Path() != "/home/user/documents" {
			t.Errorf("expected '/home/user/documents', got %s", dir.Path())
		}
	})

	t.Run("Name returns correct name", func(t *testing.T) {
		dir := newTestDir("/home/user/documents", "documents")

		if dir.Name() != "documents" {
			t.Errorf("expected 'documents', got %s", dir.Name())
		}
	})
}

func TestNode(t *testing.T) {
	t.Run("File implements Node interface", func(t *testing.T) {
		var _ Node = &File{}
	})

	t.Run("Dir implements Node interface", func(t *testing.T) {
		var _ Node = &Dir{}
	})

	t.Run("polymorphism works", func(t *testing.T) {
		var nodes []Node
		nodes = append(nodes, newTestFile("/file.txt", "file.txt"))
		nodes = append(nodes, newTestDir("/dir", "dir"))

		if len(nodes) != 2 {
			t.Errorf("expected 2 nodes, got %d", len(nodes))
		}
	})
}
