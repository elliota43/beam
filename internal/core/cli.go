package core

import (
	"fmt"
	"os"
	"path/filepath"
)

type ValidationError struct {
	Arg   string
	Cause string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("invalid argument %q: %s", e.Arg, e.Cause)
}

type PathKind int

const (
	PathFile PathKind = iota
	PathDir
)

type ParsedPath struct {
	FullPath string
	Kind     PathKind
}

func ParseArgs(args []string) ([]ParsedPath, error) {
	if len(args) == 0 {
		return nil, &ValidationError{Arg: "<files>", Cause: "no files provided"}
	}

	var out []ParsedPath

	for _, raw := range args {
		p := filepath.Clean(raw)
		info, err := os.Stat(p)
		if err != nil {
			return nil, &ValidationError{Arg: raw, Cause: "not found or not accessible"}
		}

		kind := PathFile
		if info.IsDir() {
			kind = PathDir
		}

		out = append(out, ParsedPath{FullPath: p, Kind: kind})
	}

	return out, nil
}
