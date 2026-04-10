package common

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func isTruncated(file *os.File) (bool, error) {
	currentPos, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return false, err
	}
	fileInfo, err := file.Stat()
	if err != nil {
		return false, err
	}
	return currentPos > fileInfo.Size(), nil
}

func ReadFileAt(filepath string, offset int64) ([]byte, int64, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, 0, err
	}
	defer file.Close()

	_, err = file.Seek(offset, io.SeekStart)
	if err != nil {
		return nil, 0, err
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, 0, err
	}

	endPos := offset + int64(len(data))
	return data, endPos, nil
}

func TailFile(filepath string, w io.Writer) error {
	slog.Debug("TailFile starting", "filepath", filepath)
	file, err := os.Open(filepath)
	if err != nil {
		slog.Error("TailFile failed to open file", "filepath", filepath, "error", err)
		return err
	}
	defer file.Close()

	if _, err := file.Stat(); err != nil {
		slog.Error("TailFile failed to stat file", "filepath", filepath, "error", err)
		return err
	}

	reader := bufio.NewReader(file)

	// check if the writer supports flushing
	flusher, canFlush := w.(http.Flusher)
	slog.Debug("TailFile writer flusher support", "canFlush", canFlush)

	for {
		line, err := reader.ReadString('\n')

		if err != nil {
			if err == io.EOF {
				// read a partial line, write it before waiting
				if len(line) > 0 {
					slog.Debug("TailFile writing partial line", "length", len(line))
					if _, err := io.WriteString(w, line); err != nil {
						return err
					}
					if canFlush {
						flusher.Flush()
					}
				}

				time.Sleep(100 * time.Millisecond)

				truncated, errTruncated := isTruncated(file)
				if errTruncated != nil {
					break
				}
				if truncated {
					slog.Debug("TailFile detected truncation, seeking to start")
					_, errSeekStart := file.Seek(0, io.SeekStart)
					if errSeekStart != nil {
						break
					}
					reader = bufio.NewReader(file)
				}
				continue
			}
			slog.Debug("TailFile read error", "error", err)
			break
		}

		if _, err := io.WriteString(w, line); err != nil {
			slog.Error("TailFile failed to write line", "error", err)
			return err
		}

		if canFlush {
			flusher.Flush()
		}
	}

	return nil
}

// Resolve execFile by priority: absolute path, working directory, PATH env var
func ResolveExecPath(execFile string, workingDir *string) (string, error) {
	if filepath.IsAbs(execFile) {
		if _, err := os.Stat(execFile); err == nil {
			return execFile, nil
		}
	}
	if workingDir != nil {
		workingDirCmdPath := filepath.Join(*workingDir, execFile)
		if _, err := os.Stat(workingDirCmdPath); err == nil {
			return workingDirCmdPath, nil
		}
	}
	if path, err := exec.LookPath(execFile); err == nil {
		return path, nil
	}

	return "", fmt.Errorf("Executable %q not found", execFile)
}

func CopyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}
