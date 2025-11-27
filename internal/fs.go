package internal

import (
	"fmt"
	"os"
)

func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func DirExists(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("Directory doesn't exist: %s", path)
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("Path is not a directory: %s", path)
	}
	return nil
}
