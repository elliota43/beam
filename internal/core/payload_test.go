package core

import (
	"testing"
	"time"
)

// Test helpers

func newTestPayload(nodeCount int) *Payload {
	nodes := make([]Node, nodeCount)
	for i := 0; i < nodeCount; i++ {
		nodes[i] = newTestFile("/tmp/file.txt", "file.txt")
	}
	root := Node(nodes[0])
	return NewPayload(nodes, &root)
}

func assertPayloadValid(t *testing.T, payload *Payload, expectedNodeCount int, before, after time.Time) {
	t.Helper()
	if payload == nil {
		t.Fatal("expected non-nil payload")
	}
	if len(payload.nodes) != expectedNodeCount {
		t.Errorf("expected %d nodes, got %d", expectedNodeCount, len(payload.nodes))
	}
	if payload.createdAt.Before(before) || payload.createdAt.After(after) {
		t.Error("expected createdAt to be between before and after times")
	}
}

// Tests

func TestNewPayload(t *testing.T) {
	t.Run("creates payload with correct fields", func(t *testing.T) {
		file1 := newTestFile("/tmp/file1.txt", "file1.txt")
		file2 := newTestFile("/tmp/file2.txt", "file2.txt")
		nodes := []Node{file1, file2}
		root := Node(file1)

		before := time.Now()
		payload := NewPayload(nodes, &root)
		after := time.Now()

		assertPayloadValid(t, payload, 2, before, after)

		if payload.rootNode != &root {
			t.Error("expected rootNode to match provided root")
		}
	})

	t.Run("handles empty nodes", func(t *testing.T) {
		nodes := []Node{}
		root := Node(newTestFile("/root", "root"))

		before := time.Now()
		payload := NewPayload(nodes, &root)
		after := time.Now()

		assertPayloadValid(t, payload, 0, before, after)
	})

	t.Run("handles large number of nodes", func(t *testing.T) {
		const nodeCount = 1000
		before := time.Now()
		payload := newTestPayload(nodeCount)
		after := time.Now()

		assertPayloadValid(t, payload, nodeCount, before, after)
	})
}
