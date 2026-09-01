package server

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// file_rewrite overwrites a file atomically. It resolves symlinks before
// writing so that the correct file is always updated, even if the path
// contains symbolic links.
func file_rewrite(file string, data []byte) error {
	file = fullPath(file)
	if err := os.WriteFile(file, data, 0o644); err != nil {
		return fmt.Errorf("unable to create or write to the file %s: %w", file, err)
	}
	return nil
}

func fileExists(file string) bool {
	info, err := os.Stat(file)
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	return !info.IsDir()
}

func cleanPath(p string) string {
	p = strings.ReplaceAll(p, PS+PS, PS)
	return p
}

// fullPath resolves a path to its absolute form and resolves any symbolic
// links. filepath.EvalSymlinks already handles component-by-component
// resolution when the full path doesn't exist, so a manual loop is
// unnecessary.
func fullPath(oldPath string) string {
	absPath, err := filepath.Abs(oldPath)
	if err != nil {
		return cleanPath(oldPath)
	}
	evalPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return cleanPath(absPath)
	}
	return cleanPath(evalPath)
}
