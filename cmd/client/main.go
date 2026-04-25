package main

import (
	"flag"
	"fmt"
)

func main() {
	flag.Parse()

	args := flag.Args()

	fmt.Println(args)
}
