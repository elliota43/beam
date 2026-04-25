package upload

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateUploadSingleFile(t *testing.T) {
	storageDir := t.TempDir()
	h := NewHandler("http://example.com", storageDir)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("files", "hello.txt")
	if err != nil {
		t.Fatal(err)
	}

	content := []byte("hello beam")
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/uploads", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rr := httptest.NewRecorder()

	h.CreateUpload(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}

	var resp UploadResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	if resp.URL == "" {
		t.Fatal("expected upload URL")
	}

	if len(resp.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(resp.Files))
	}

	gotFile := resp.Files[0]

	if gotFile.Name != "hello.txt" {
		t.Fatalf("expected original filename hello.txt, got %q", gotFile.Name)
	}

	if gotFile.Size != int64(len(content)) {
		t.Fatalf("expected size %d, got %d", len(content), gotFile.Size)
	}

	wantHashBytes := sha256.Sum256(content)
	wantHash := hex.EncodeToString(wantHashBytes[:])

	if gotFile.Hash != wantHash {
		t.Fatalf("expected hash %s, got %s", wantHash, gotFile.Hash)
	}

	entries, err := os.ReadDir(storageDir)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected one upload directory, got %d", len(entries))
	}

	uploadDir := filepath.Join(storageDir, entries[0].Name())

	if _, err := os.Stat(filepath.Join(uploadDir, MetadataFileName)); err != nil {
		t.Fatalf("expected metadata file: %v", err)
	}

	meta, err := readMetadata(uploadDir)
	if err != nil {
		t.Fatal(err)
	}

	if meta.Slug == "" {
		t.Fatal("expected metadata slug")
	}

	if len(meta.Files) != 1 {
		t.Fatalf("expected 1 metadata file, got %d", len(meta.Files))
	}

	if meta.Files[0].OriginalName != "hello.txt" {
		t.Fatalf("expected original name hello.txt, got %q", meta.Files[0].OriginalName)
	}

	if meta.Files[0].SHA256 != wantHash {
		t.Fatalf("expected metadata hash %s, got %s", wantHash, meta.Files[0].SHA256)
	}
}

func TestCreateUploadMultipleFiles(t *testing.T) {
	storageDir := t.TempDir()
	h := NewHandler("http://example.com", storageDir)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	files := map[string]string{
		"a.txt": "aaa",
		"b.txt": "bbb",
	}

	for name, content := range files {
		part, err := writer.CreateFormFile("files", name)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := part.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/uploads", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rr := httptest.NewRecorder()
	h.CreateUpload(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}

	var resp UploadResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	if len(resp.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(resp.Files))
	}

}

func TestCreateUploadRejectsNonPost(t *testing.T) {
	h := NewHandler("http://example.com", t.TempDir())

	req := httptest.NewRequest(http.MethodGet, "/api/uploads", nil)
	rr := httptest.NewRecorder()

	h.CreateUpload(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected %d, got %d", http.StatusMethodNotAllowed, rr.Code)
	}
}

func TestCreateUploadRejectsNoFiles(t *testing.T) {
	h := NewHandler("http://example.com", t.TempDir())

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/uploads", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rr := httptest.NewRecorder()

	h.CreateUpload(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestCreateUploadRejectsTooLargeFile(t *testing.T) {
	storageDir := t.TempDir()
	h := NewHandler("http://example.com", storageDir)
	h.MaxFileSize = 3

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("files", "large.txt")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := part.Write([]byte("too large")); err != nil {
		t.Fatal(err)
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/uploads", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rr := httptest.NewRecorder()

	h.CreateUpload(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}

	entries, err := os.ReadDir(storageDir)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 0 {
		t.Fatalf("expected failed upload to clean up storage dir, found %d entries", len(entries))
	}
}

func TestServeUploadRedirectsSingleFileUpload(t *testing.T) {
	storageDir := t.TempDir()
	h := NewHandler("http://example.com", storageDir)

	uploadDir := filepath.Join(storageDir, "abc123")
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(uploadDir, "stored-file"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	meta := UploadMetadata{
		Slug: "abc123",
		Files: []FileMetadata{
			{
				OriginalName: "hello.txt",
				StoredName:   "stored-file",
				Size:         5,
				ContentType:  "text/plain",
			},
		},
	}

	if err := writeMetadata(uploadDir, meta); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/u/abc123", nil)
	rr := httptest.NewRecorder()

	h.ServeUpload(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected %d, got %d", http.StatusFound, rr.Code)
	}

	location := rr.Header().Get("Location")
	if location != "/u/abc123/hello.txt" {
		t.Fatalf("expected redirect to /u/abc123/hello.txt, got %q", location)
	}
}

func TestServeUploadServesStoredFile(t *testing.T) {
	storageDir := t.TempDir()
	h := NewHandler("http://example.com", storageDir)

	uploadDir := filepath.Join(storageDir, "abc123")
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		t.Fatal(err)
	}

	content := []byte("hello from storage")

	if err := os.WriteFile(filepath.Join(uploadDir, "stored-file"), content, 0644); err != nil {
		t.Fatal(err)
	}

	meta := UploadMetadata{
		Slug: "abc123",
		Files: []FileMetadata{
			{
				OriginalName: "hello.txt",
				StoredName:   "stored-file",
				Size:         int64(len(content)),
				ContentType:  "text/plain",
			},
		},
	}

	if err := writeMetadata(uploadDir, meta); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/u/abc123/hello.txt", nil)
	rr := httptest.NewRecorder()

	h.ServeUpload(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	if rr.Body.String() != string(content) {
		t.Fatalf("expected body %q, got %q", string(content), rr.Body.String())
	}

	disposition := rr.Header().Get("Content-Disposition")
	if disposition == "" {
		t.Fatal("expected Content-Disposition header")
	}
}

func TestServeUploadRendersFileListForMultipleFiles(t *testing.T) {
	storageDir := t.TempDir()
	h := NewHandler("http://example.com", storageDir)

	uploadDir := filepath.Join(storageDir, "abc123")
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		t.Fatal(err)
	}

	meta := UploadMetadata{
		Slug: "abc123",
		Files: []FileMetadata{
			{OriginalName: "a.txt", StoredName: "stored-a", Size: 3},
			{OriginalName: "b.txt", StoredName: "stored-b", Size: 3},
		},
	}

	if err := writeMetadata(uploadDir, meta); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/u/abc123", nil)
	rr := httptest.NewRecorder()

	h.ServeUpload(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rr.Code)
	}

	body := rr.Body.String()

	if !strings.Contains(body, "a.txt") {
		t.Fatal("expected rendered file list to contain a.txt")
	}

	if !strings.Contains(body, "b.txt") {
		t.Fatal("expected rendered file list to contain b.txt")
	}
}

func TestServeUploadReturnsNotFoundForMissingUpload(t *testing.T) {
	h := NewHandler("http://example.com", t.TempDir())

	req := httptest.NewRequest(http.MethodGet, "/u/does-not-exist", nil)
	rr := httptest.NewRecorder()

	h.ServeUpload(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestServeUploadReturnsNotFoundForMissingFile(t *testing.T) {
	storageDir := t.TempDir()
	h := NewHandler("http://example.com", storageDir)

	uploadDir := filepath.Join(storageDir, "abc123")
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		t.Fatal(err)
	}

	meta := UploadMetadata{
		Slug: "abc123",
		Files: []FileMetadata{
			{OriginalName: "a.txt", StoredName: "stored-a", Size: 3},
		},
	}

	if err := writeMetadata(uploadDir, meta); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/u/abc123/nope.txt", nil)
	rr := httptest.NewRecorder()

	h.ServeUpload(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestWriteAndReadMetadata(t *testing.T) {
	dir := t.TempDir()

	want := UploadMetadata{
		Slug: "abc123",
		Files: []FileMetadata{
			{
				OriginalName: "hello.txt",
				StoredName:   "stored-file",
				Size:         123,
				ContentType:  "text/plain",
				SHA256:       "fakehash",
			},
		},
	}

	if err := writeMetadata(dir, want); err != nil {
		t.Fatal(err)
	}

	got, err := readMetadata(dir)
	if err != nil {
		t.Fatal(err)
	}

	if got.Slug != want.Slug {
		t.Fatalf("expected slug %q, got %q", want.Slug, got.Slug)
	}

	if len(got.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(got.Files))
	}

	if got.Files[0].OriginalName != "hello.txt" {
		t.Fatalf("expected original name hello.txt, got %q", got.Files[0].OriginalName)
	}
}

func TestRandomSlug(t *testing.T) {
	slug, err := randomSlug(URLSlugLength)
	if err != nil {
		t.Fatal(err)
	}

	if slug == "" {
		t.Fatal("expected non-empty slug")
	}

	other, err := randomSlug(URLSlugLength)
	if err != nil {
		t.Fatal(err)
	}

	if slug == other {
		t.Fatal("expected two random slugs to differ")
	}
}
