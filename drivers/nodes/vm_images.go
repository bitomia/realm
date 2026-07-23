//go:build ignore

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
	"strconv"
	"strings"

	"github.com/digitalocean/go-libvirt"

	"github.com/google/uuid"

	"github.com/bitomia/realm/agent/config"
	"github.com/bitomia/realm/common"
)

const (
	defaultLibVirtSocket = "/var/run/libvirt/libvirt-sock"
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

func createDrives(drives []VMDrive, nodeName, socket string) (map[int]OverlayImage, error) {
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

		if drives[i].Resize != "" {
			if err := resizeOverlay(overlayImage.FilePath, drives[i].Resize, socket); err != nil {
				overlayImage.Cleanup()
				return nil, err
			}
		}

		overlays[i] = *overlayImage
	}
	return overlays, nil
}

func parseDriveSize(size string) (uint64, error) {
	s := strings.TrimSpace(size)
	if s == "" {
		return 0, fmt.Errorf("empty size")
	}
	mult := uint64(1)
	switch s[len(s)-1] {
	case 'k', 'K':
		mult = 1 << 10
		s = s[:len(s)-1]
	case 'm', 'M':
		mult = 1 << 20
		s = s[:len(s)-1]
	case 'g', 'G':
		mult = 1 << 30
		s = s[:len(s)-1]
	case 't', 'T':
		mult = 1 << 40
		s = s[:len(s)-1]
	}
	n, err := strconv.ParseUint(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size %q: %w", size, err)
	}
	return n * mult, nil
}

func resizeOverlay(filePath, size, socket string) error {
	capacity, err := parseDriveSize(size)
	if err != nil {
		return fmt.Errorf("vm: invalid resize for %s: %w", filePath, err)
	}

	slog.Info("vm_images.resizeOverlay", "msg", "resizing overlay", "path", filePath, "size", size, "bytes", capacity)

	dir := filepath.Dir(filePath)
	volName := filepath.Base(filePath)
	poolName := fmt.Sprintf("realm-resize-%s", uuid.New().String())
	poolXML := fmt.Sprintf(`<pool type='dir'><name>%s</name><target><path>%s</path></target></pool>`, poolName, dir)

	return withLibvirt(socket, func(l *libvirt.Libvirt) error {
		pool, err := l.StoragePoolCreateXML(poolXML, 0)
		if err != nil {
			return fmt.Errorf("vm: failed to create transient storage pool: %w", err)
		}
		defer func() {
			if err := l.StoragePoolDestroy(pool); err != nil {
				slog.Warn("vm_images.resizeOverlay", "msg", "failed to destroy transient pool", "pool", poolName, "error", err)
			}
		}()

		if err := l.StoragePoolRefresh(pool, 0); err != nil {
			return fmt.Errorf("vm: failed to refresh storage pool: %w", err)
		}

		vol, err := l.StorageVolLookupByName(pool, volName)
		if err != nil {
			return fmt.Errorf("vm: failed to lookup overlay volume %s: %w", volName, err)
		}

		if err := l.StorageVolResize(vol, capacity, 0); err != nil {
			return fmt.Errorf("vm: StorageVolResize failed: %w", err)
		}
		return nil
	})
}

func cleanupOverlays(nodeName string) {
	dir := filepath.Join(config.Get().DataPath, "overlays", nodeName)
	if err := os.RemoveAll(dir); err != nil {
		slog.Warn("vm_images.cleanupOverlays", "msg", "failed to clean up overlays", "node", nodeName, "error", err)
	}
}
