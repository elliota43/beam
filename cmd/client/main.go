package main

// Usage:
// go run ./cmd/client ./README.md
// go run ./cmd/client -server http://localhost:9001 ./README.md

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

type uploadResponse struct {
	URL   string `json:"url"`
	Files []struct {
		Name   string `json:"name"`
		Size   int64  `json:"size"`
		URL    string `json:"url"`
		SHA256 string `json:"sha256"`
	} `json:"files"`
}

func main() {
	server := flag.String("server", "http://localhost:9001", "beam server URL")
	flag.Parse()

	paths := flag.Args()
	if len(paths) == 0 {
		fmt.Fprintf(os.Stderr, "usage: beam [-server http://localhost:9001] <file> [file...]\n")
		os.Exit(2)
	}

	resp, err := uploadFiles(*server, paths)
	if err != nil {
		fmt.Fprintf(os.Stderr, "upload failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(resp.URL)

	for _, f := range resp.Files {
		fmt.Printf("- %s (%d bytes): %s\n", f.Name, f.Size, f.URL)
	}
}

func uploadFiles(server string, paths []string) (uploadResponse, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	for _, path := range paths {
		if err := addFile(writer, path); err != nil {
			return uploadResponse{}, err
		}
	}

	if err := writer.Close(); err != nil {
		return uploadResponse{}, err
	}

	req, err := http.NewRequest(http.MethodPost, server+"/api/uploads", &body)
	if err != nil {
		return uploadResponse{}, err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return uploadResponse{}, err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return uploadResponse{}, fmt.Errorf("server returned %s: %s", res.Status, bytes.TrimSpace(msg))
	}

	var out uploadResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return uploadResponse{}, err
	}

	return out, nil
}

func addFile(writer *multipart.Writer, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}

	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}

	if info.IsDir() {
		return fmt.Errorf("%s is a directory; this prototype only accepts explicit files right now", path)
	}

	part, err := writer.CreateFormFile("files", filepath.Base(path))
	if err != nil {
		return err
	}

	_, err = io.Copy(part, f)
	return err
}
