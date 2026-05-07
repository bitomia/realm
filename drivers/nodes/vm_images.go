package nodes

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/bitomia/realm/agent/config"
	"github.com/bitomia/realm/common"
)

type OverlayImage struct {
	ID       uuid.UUID
	FilePath string
}

func CreateOverlay(nodeName, imagePath string) (*OverlayImage, error) {
	oDir, err := overlaysDir(nodeName)
	if err != nil {
		return nil, err
	}

	var overlayImage OverlayImage
	overlayImage.ID = uuid.New()
	overlayImage.FilePath, err = filepath.Abs(filepath.Join(oDir, overlayImage.ID.String()))
	if err != nil {
		return nil, fmt.Errorf("vm: failed to resolve overlay absolute path: %w", err)
	}

	if err := common.CopyFile(imagePath, overlayImage.FilePath); err != nil {
		return nil, err
	}

	return &overlayImage, nil
}

func (o *OverlayImage) Cleanup() {
	slog.Info("OverlayImage.Cleanup", "msg", "cleaning up overlay image", "path", o.FilePath)
	if err := os.Remove(o.FilePath); err != nil {
		slog.Warn("OverlayImage.Cleanup", "msg", "failed to clean up overlay", "error", err)
	}
	o.ID = uuid.Nil
	o.FilePath = ""
}

func isURLDrive(file string) bool {
	return strings.HasPrefix(file, "http://") || strings.HasPrefix(file, "https://")
}

func imagesCacheDir() (string, error) {
	dir := filepath.Join(config.Get().DataPath, "images")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("vm: failed to create images cache directory: %w", err)
	}
	return dir, nil
}

func overlaysDir(nodeName string) (string, error) {
	dir := filepath.Join(config.Get().DataPath, "overlays", nodeName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("vm: failed to create overlays directory: %w", err)
	}
	return dir, nil
}

func urlToCacheFilename(rawURL string) string {
	hash := sha256.Sum256([]byte(rawURL))
	return hex.EncodeToString(hash[:])
}

func downloadImage(rawURL string) (string, error) {
	cacheDir, err := imagesCacheDir()
	if err != nil {
		return "", err
	}

	cachedPath := filepath.Join(cacheDir, urlToCacheFilename(rawURL))

	if _, err := os.Stat(cachedPath); err == nil {
		slog.Info("vm_images.downloadImage", "msg", "using cached image", "url", rawURL, "path", cachedPath)
		return cachedPath, nil
	}

	slog.Info("vm_images.downloadImage", "msg", "downloading image", "url", rawURL)

	tmpFile, err := os.CreateTemp(cacheDir, ".tmp.*")
	if err != nil {
		return "", fmt.Errorf("vm: failed to create temp file for download: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		tmpFile.Close()
		os.Remove(tmpPath)
	}()

	resp, err := http.Get(rawURL)
	if err != nil {
		return "", fmt.Errorf("vm: failed to download image from %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("vm: failed to download image from %s: HTTP %d", rawURL, resp.StatusCode)
	}

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		return "", fmt.Errorf("vm: failed to write downloaded image: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("vm: failed to close downloaded image: %w", err)
	}

	if err := os.Rename(tmpPath, cachedPath); err != nil {
		// another process may have placed the file already
		if _, statErr := os.Stat(cachedPath); statErr == nil {
			return cachedPath, nil
		}
		return "", fmt.Errorf("vm: failed to move downloaded image to cache: %w", err)
	}

	slog.Info("vm_images.downloadImage", "msg", "image downloaded", "url", rawURL, "path", cachedPath)
	return cachedPath, nil
}

func resolveDrives(drives []VMDrive, nodeName string) (map[int]OverlayImage, error) {
	overlays := make(map[int]OverlayImage)
	for i := range drives {
		imagePath := drives[i].File
		if isURLDrive(imagePath) {
			var err error
			if imagePath, err = downloadImage(drives[i].File); err != nil {
				return nil, err
			}
		}

		overlayImage, err := CreateOverlay(nodeName, imagePath)
		if err != nil {
			return nil, err
		}

		overlays[i] = *overlayImage
	}
	return overlays, nil
}

func cleanupOverlays(nodeName string) {
	dir := filepath.Join(config.Get().DataPath, "overlays", nodeName)
	if err := os.RemoveAll(dir); err != nil {
		slog.Warn("vm_images.cleanupOverlays", "msg", "failed to clean up overlays", "node", nodeName, "error", err)
	}
}
