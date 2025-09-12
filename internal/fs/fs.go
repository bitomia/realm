package fs

import (
	"fmt"
	"os"
	"path/filepath"
)

type PathKind int

const (
	PathNone PathKind = iota
	PathFile
	PathDir
)

func MkdirAll(path string, perm os.FileMode) error {
	if err := os.MkdirAll(path, perm); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", path, err)
	}
	return nil
}

func MkdirBinAll(path string, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Join(BinDir(), path), perm); err != nil {
		panic(fmt.Sprintf("failed to create binary directory %s: %v", path, err))
	}
	return nil
}

func TouchFile(path string, perm os.FileMode) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, perm)
		if err != nil {
			return fmt.Errorf("failed to create file %s: %w", path, err)
		}
		defer file.Close()
	}
	return nil
}

func BinDir() string {
	bin, err := os.Executable()
	if err != nil {
		panic(fmt.Sprintf("Could not determine executable path: %v", err))
	}
	binDir := filepath.Dir(bin)
	return binDir
}

func GetBinPath(join ...string) (string, error) {
	binDir := BinDir()

	path := filepath.Join(binDir, filepath.Join(join...))

	kind, err := ClassifyPath(path)

	if err != nil {
		return path, err
	}

	switch kind {
	case PathFile:
		if !FileExists(path) {
			return path, fmt.Errorf("expected file but not found: %s", path)
		}
	case PathDir:
		if !DirExists(path) {
			return path, fmt.Errorf("expected directory but not found: %s", path)
		}
	default:
		return path, fmt.Errorf("unknown path type: %s", path)
	}

	return path, nil
}

func CreateDirIfNotExists(path string) error {
	if !DirExists(path) {
		if err := MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", path, err)
		}
	}
	return nil
}

func DirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func IsDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func IsFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular()
}

func ClassifyPath(path string) (PathKind, error) {
	cs, err := filepath.Abs(path)
	if err != nil {
		return PathNone, err
	}

	if !FileExists(cs) {
		return PathNone, nil
	}

	switch {
	case IsDir(cs):
		return PathDir, nil
	case IsFile(cs):
		return PathFile, nil
	default:
		return PathNone, fmt.Errorf("unsupported path type: %s", cs)
	}
}
