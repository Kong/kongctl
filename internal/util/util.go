package util

import (
	"io/fs"
	"os"
	"path/filepath"
)

// InitDir initializes a directory with the given mode
func InitDir(path string, mode fs.FileMode) error {
	expandedDir := os.ExpandEnv(path)
	fullPath := filepath.Dir(expandedDir)
	err := os.MkdirAll(fullPath, mode)
	return err
}

func GetString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
