package internal

import (
	"fmt"
	"os"
	"path/filepath"
)

type LogsPath string

// Create a file and all the missing directories
func CreateLogFile(logsPath LogsPath, filename string, perm os.FileMode) (*os.File, error) {
	if err := os.MkdirAll(string(logsPath), perm); err != nil {
		return nil, fmt.Errorf("Failed to create directories: %v", err)
	}

	filepath := filepath.Join(string(logsPath), filename)
	file, err := os.Create(filepath)
	if err != nil {
		return nil, fmt.Errorf("Failed to create file: %v", err)
	}

	return file, nil
}

const bytesToMB = 1024.0 * 1024.0

func ToMB(bytes float64) float64 {
	return bytes / bytesToMB
}
