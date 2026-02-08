package core

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func (ft *Filetree) ToZipBytes() ([]byte, error) {
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	root := *ft.Root
	if err := compressNode(zipWriter, root, ""); err != nil {
		zipWriter.Close()
		return nil, err
	}

	if err := zipWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to close zip writer: %w", err)
	}

	return buf.Bytes(), nil
}

func compressNode(zw *zip.Writer, node Node, basePath string) error {
	archivePath := filepath.Join(basePath, node.Name())

	switch n := node.(type) {
	case *File:
		return addFileToZip(zw, n.Path(), archivePath)
	case *Dir:
		for _, child := range n.Children() {
			if err := compressNode(zw, child, archivePath); err != nil {
				return err
			}
		}
	}
	return nil
}

func addFileToZip(zw *zip.Writer, srcPath, archivePath string) error {
	file, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", srcPath, err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return fmt.Errorf("failed to create zip header: %w", err)
	}
	header.Name = archivePath
	header.Method = zip.Deflate

	writer, err := zw.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("failed to create zip entry: %w", err)
	}

	if _, err := io.Copy(writer, file); err != nil {
		return fmt.Errorf("failed to write file to zip: %w", err)
	}

	return nil
}

func (ft *Filetree) GetUncompressedSize() (int64, error) {
	var totalSize int64
	nodes := ft.FlattenTree()

	for _, node := range nodes {
		if file, ok := node.(*File); ok {
			info, err := os.Stat(file.Path())
			if err != nil {
				return 0, err
			}
			totalSize += info.Size()
		}
	}

	return totalSize, nil
}
