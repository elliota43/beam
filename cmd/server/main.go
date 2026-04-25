package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/elliota43/beam/internal/upload"
)

func main() {
	addr := flag.String("addr", ":9001", "server listen address")
	baseURL := flag.String("base-url", "http://localhost:9001", "public base URL used in returned links")
	storageDir := flag.String("storage", "./data/uploads", "directory where uploaded files are stored")
	flag.Parse()

	h := upload.NewHandler(*baseURL, *storageDir)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/uploads", h.CreateUpload)
	mux.HandleFunc("GET /u/", h.ServeUpload)

	log.Printf("beam server listening on%s", *addr)
	log.Printf("storing uploads in %s", *storageDir)

	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
}
