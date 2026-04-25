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
	"time"
)

const (
	URLSlugLength     = 8
	StorageSlugLength = 12
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

	resp := UploadResponse{
		URL: fmt.Sprintf("%s/u/%s", h.BaseURL, slug),
	}

	for _, fh := range files {
		fileResp, err := h.saveUploadedFile(slug, uploadDir, fh)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		resp.Files = append(resp.Files, fileResp)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) saveUploadedFile(slug, uploadDir string, fh *multipart.FileHeader) (FileResponse, error) {
	if fh.Size > h.MaxFileSize {
		return FileResponse{}, fmt.Errorf("file too large: %s", fh.Filename)
	}

	src, err := fh.Open()
	if err != nil {
		return FileResponse{}, fmt.Errorf("failed to open uploaded file")
	}

	defer src.Close()

	fileID, err := randomSlug(StorageSlugLength)
	if err != nil {
		return FileResponse{}, fmt.Errorf("failed to generate file id")
	}

	storedPath := filepath.Join(uploadDir, fileID)

	dst, err := os.OpenFile(storedPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
	if err != nil {
		return FileResponse{}, fmt.Errorf("failed to create stored file")
	}

	defer dst.Close()

	hasher := sha256.New()

	n, err := io.Copy(io.MultiWriter(dst, hasher), src)
	if err != nil {
		return FileResponse{}, fmt.Errorf("failed to save uploaded file")
	}

	if n > h.MaxFileSize {
		_ = os.Remove(storedPath)
		return FileResponse{}, fmt.Errorf("file too large: %s", fh.Filename)
	}

	hash := hex.EncodeToString(hasher.Sum(nil))

	return FileResponse{
		Name: filepath.Base(fh.Filename),
		Size: n,
		URL:  fmt.Sprintf("%s/u/%s/%s", h.BaseURL, slug, filepath.Base(fh.Filename)),
		Hash: hash,
	}, nil
}

func randomSlug(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func NewHandler(baseURL, storageDir string) *Handler {
	return &Handler{
		BaseURL:     baseURL,
		StorageDir:  storageDir,
		MaxFileSize: 100 << 20,
	}
}
