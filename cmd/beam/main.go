package main

import (
	"flag"
	"fmt"
	"path/filepath"
)

func main() {
	flag.Parse()

	for _, path := range flag.Args() {
		absPath, _ := filepath.Abs(path)
		fmt.Println("Processing:", absPath)
	}
}
