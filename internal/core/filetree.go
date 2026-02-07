package core

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Filetree struct {
	Root *Node
}

func BuildFiletree(paths []ParsedPath) (*Filetree, error) {
	var rootNodes []Node

	for _, parsedPath := range paths {
		if parsedPath.Kind == PathDir {
			dirNode, err := buildDirTree(parsedPath.FullPath)
			if err != nil {
				return nil, err
			}
			rootNodes = append(rootNodes, dirNode)
		} else {
			fileNode := &File{
				path: parsedPath.FullPath,
				name: filepath.Base(parsedPath.FullPath),
			}
			rootNodes = append(rootNodes, fileNode)
		}
	}

	if len(rootNodes) == 0 {
		return nil, fmt.Errorf("no valid paths provided")
	}

	// determine root
	var root Node
	if len(rootNodes) == 1 {
		root = rootNodes[0]
	} else {
		root = createVirtualRoot(rootNodes)
	}

	return &Filetree{
		Root: &root,
	}, nil
}

func buildDirTree(dirPath string) (*Dir, error) {
	dir := &Dir{
		path:     dirPath,
		name:     filepath.Base(dirPath),
		children: []Node{},
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		childPath := filepath.Join(dirPath, entry.Name())

		if entry.IsDir() {
			childDir, err := buildDirTree(childPath)
			if err != nil {
				return nil, err
			}
			childDir.parent = dir
			dir.children = append(dir.children, childDir)
		} else {
			childFile := &File{
				path: childPath,
				name: entry.Name(),
				dir:  dir,
			}
			dir.children = append(dir.children, childFile)
		}
	}

	return dir, nil
}

func createVirtualRoot(children []Node) *Dir {
	virtualRoot := &Dir{
		path:     fmt.Sprintf("upload_%s", time.Now().Format("2006_01_02_150405")),
		name:     fmt.Sprintf("upload_%s", time.Now().Format("2006_01_02_150405")),
		children: children,
	}

	for _, child := range children {
		if dir, ok := child.(*Dir); ok {
			dir.parent = virtualRoot
		} else if file, ok := child.(*File); ok {
			file.dir = virtualRoot
		}
	}

	return virtualRoot
}
