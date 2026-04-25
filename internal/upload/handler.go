package upload

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	URLSlugLength     = 8
	StorageSlugLength = 12
	MetadataFileName  = "metadata.json"
)

type Handler struct {
	BaseURL     string
	StorageDir  string
	MaxFileSize int64
}

type UploadResponse struct {
	URL   string         `json:"url"`
	Files []FileResponse `json:"files"`
}

type FileResponse struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	URL  string `json:"url"`
	Hash string `json:"sha256"`
}

type UploadMetadata struct {
	Slug      string         `json:"slug"`
	CreatedAt time.Time      `json:"created_at"`
	Files     []FileMetadata `json:"files"`
}

type FileMetadata struct {
	OriginalName string    `json:"original_name"`
	StoredName   string    `json:"stored_name"`
	Size         int64     `json:"size"`
	ContentType  string    `json:"content_type"`
	SHA256       string    `json:"sha256"`
	CreatedAt    time.Time `json:"created_at"`
}

func NewHandler(baseURL, storageDir string) *Handler {
	return &Handler{
		BaseURL:     baseURL,
		StorageDir:  storageDir,
		MaxFileSize: 100 << 20,
	}
}

func (h *Handler) CreateUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, h.MaxFileSize*10)

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "invalid multipart upload", http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		http.Error(w, "no files provided", http.StatusBadRequest)
		return
	}

	slug, err := randomSlug(URLSlugLength)
	if err != nil {
		http.Error(w, "failed to generate upload id", http.StatusInternalServerError)
		return
	}

	uploadDir := filepath.Join(h.StorageDir, slug)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		http.Error(w, "failed to create upload directory", http.StatusInternalServerError)
		return
	}

	meta := UploadMetadata{
		Slug:      slug,
		CreatedAt: time.Now().UTC(),
	}

	resp := UploadResponse{
		URL: fmt.Sprintf("%s/u/%s", h.BaseURL, slug),
	}

	for _, fh := range files {
		fileMeta, fileResp, err := h.saveUploadedFile(slug, uploadDir, fh)
		if err != nil {
			_ = os.RemoveAll(uploadDir)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		meta.Files = append(meta.Files, fileMeta)
		resp.Files = append(resp.Files, fileResp)
	}

	if err := writeMetadata(uploadDir, meta); err != nil {
		_ = os.RemoveAll(uploadDir)
		http.Error(w, "failed to persist upload metadata", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) saveUploadedFile(slug, uploadDir string, fh *multipart.FileHeader) (FileMetadata, FileResponse, error) {
	if fh.Size > h.MaxFileSize {
		return FileMetadata{}, FileResponse{}, fmt.Errorf("file too large: %s", fh.Filename)
	}

	src, err := fh.Open()
	if err != nil {
		return FileMetadata{}, FileResponse{}, fmt.Errorf("failed to open uploaded file")
	}
	defer src.Close()

	storedName, err := randomSlug(StorageSlugLength)
	if err != nil {
		return FileMetadata{}, FileResponse{}, fmt.Errorf("failed to generate stored filename")
	}

	storedPath := filepath.Join(uploadDir, storedName)

	dst, err := os.OpenFile(storedPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
	if err != nil {
		return FileMetadata{}, FileResponse{}, fmt.Errorf("failed to create stored file")
	}

	defer dst.Close()

	hasher := sha256.New()

	n, err := io.Copy(io.MultiWriter(dst, hasher), src)
	if err != nil {
		_ = os.Remove(storedPath)
		return FileMetadata{}, FileResponse{}, fmt.Errorf("failed to save uploaded file")
	}

	if n > h.MaxFileSize {
		_ = os.Remove(storedPath)
		return FileMetadata{}, FileResponse{}, fmt.Errorf("file too large: %s", fh.Filename)
	}

	originalName := filepath.Base(fh.Filename)
	hash := hex.EncodeToString(hasher.Sum(nil))
	createdAt := time.Now().UTC()

	fileMeta := FileMetadata{
		OriginalName: originalName,
		StoredName:   storedName,
		Size:         n,
		ContentType:  fh.Header.Get("Content-Type"),
		SHA256:       hash,
		CreatedAt:    createdAt,
	}

	fileResp := FileResponse{
		Name: originalName,
		Size: n,
		URL:  fmt.Sprintf("%s/u/%s/%s", h.BaseURL, slug, originalName),
		Hash: hash,
	}

	return fileMeta, fileResp, nil
}

func (h *Handler) ServeUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// supports:
	// GET /u/{slug}
	// GET /u/{slug}/{filename}
	path := strings.TrimPrefix(r.URL.Path, "/u/")
	parts := strings.SplitN(path, "/", 2)

	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}

	slug := parts[0]
	uploadDir := filepath.Join(h.StorageDir, slug)

	meta, err := readMetadata(uploadDir)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if len(parts) == 1 || parts[1] == "" {
		if len(meta.Files) == 1 {
			http.Redirect(w, r, "/u/"+slug+"/"+meta.Files[0].OriginalName, http.StatusFound)
			return
		}

		h.renderFileList(w, meta)
		return
	}

	requestedName := filepath.Base(parts[1])

	for _, f := range meta.Files {
		if f.OriginalName == requestedName {
			storedPath := filepath.Join(uploadDir, f.StoredName)

			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", f.OriginalName))

			if f.ContentType != "" {
				w.Header().Set("Content-Type", f.ContentType)
			}

			http.ServeFile(w, r, storedPath)
			return
		}
	}

	http.NotFound(w, r)
}

func (h *Handler) renderFileList(w http.ResponseWriter, meta UploadMetadata) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	fmt.Fprintf(w, "<h1>beam upload: %s</h1>", meta.Slug)
	fmt.Fprintln(w, "<ul>")

	for _, f := range meta.Files {
		fmt.Fprintf(
			w,
			`<li><a href="/u/%s/%s">%s</a> (%d bytes)</li>`,
			meta.Slug,
			f.OriginalName,
			f.OriginalName,
			f.Size,
		)
	}

	fmt.Fprintln(w, "</ul>")
}

func writeMetadata(uploadDir string, meta UploadMetadata) error {
	path := filepath.Join(uploadDir, MetadataFileName)

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")

	return enc.Encode(meta)
}

func readMetadata(uploadDir string) (UploadMetadata, error) {
	path := filepath.Join(uploadDir, MetadataFileName)

	f, err := os.Open(path)
	if err != nil {
		return UploadMetadata{}, err
	}

	defer f.Close()

	var meta UploadMetadata

	if err := json.NewDecoder(f).Decode(&meta); err != nil {
		return UploadMetadata{}, err
	}
	return meta, nil
}

func randomSlug(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}
