package main

import (
	"beam/internal/core"
	"fmt"
	"os"
)

func main() {
	args := os.Args[1:]

	parsedPaths, err := core.ParseArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	filetree, err := core.BuildFiletree(parsedPaths)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error building filetree: %v\n", err)
		os.Exit(1)
	}

	// flatten tree & create payload
	nodes := filetree.FlattenTree()
	payload := core.NewPayload(nodes, filetree.Root)

	zipBytes, err := filetree.ToZipBytes()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error compressing: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Compressed to %d bytes\n", len(zipBytes))
	fmt.Println("\nPayload ready for transmission!")

	_ = payload
}
