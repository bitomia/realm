package common

import (
	"fmt"
	"os"
	"path/filepath"
)

// Create a file and all the missing directories
func CreateLogFile(filePath string, perm os.FileMode) (*os.File, error) {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, perm); err != nil {
		return nil, fmt.Errorf("Failed to create directory: %v", err)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("Failed to create file: %v", err)
	}

	return file, nil
}
