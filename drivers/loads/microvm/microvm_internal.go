//go:build linux

package microvm

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/bitomia/realm/agent/config"
	"github.com/bitomia/realm/common"
)

type BootPlan struct {
	KernelPath string
	InitrdPath string
	BootArgs   string
	Drives     map[int]OverlayImage
}

const (
	defaultMemoryMiB = 512
	defaultVCPUs     = 1

	// defaultBootArgs is the conventional Firecracker command line: no PCI, no
	// reboot handling in the guest, and a serial console on the VMM's stdout.
	defaultBootArgs = "console=ttyS0 reboot=k panic=1 pci=off"

	// apiReadyTimeout bounds the wait for firecracker to bind its API socket.
	apiReadyTimeout = 10 * time.Second

	// gracefulShutdownTimeout bounds the wait for a guest to halt after an
	// ACPI power button press before the VMM is killed.
	gracefulShutdownTimeout = 30 * time.Second

	// killTimeout bounds the wait for a killed VMM to disappear.
	killTimeout = 5 * time.Second

	actionInstanceStart  = "InstanceStart"
	actionSendCtrlAltDel = "SendCtrlAltDel"
)

// OverlayImage is a per-load writable copy of a base image, so a cached base
// image is never mutated by a running microVM.
type OverlayImage struct {
	ID       uuid.UUID
	FilePath string
}

func CreateOverlay(loadName, imagePath string) (*OverlayImage, error) {
	oDir, err := overlaysDir(loadName)
	if err != nil {
		return nil, err
	}

	var overlayImage OverlayImage
	overlayImage.ID = uuid.New()
	overlayImage.FilePath, err = filepath.Abs(filepath.Join(oDir, overlayImage.ID.String()))
	if err != nil {
		return nil, fmt.Errorf("MicroVMDriver: failed to resolve overlay absolute path: %w", err)
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

func isURLImage(file string) bool {
	return strings.HasPrefix(file, "http://") || strings.HasPrefix(file, "https://")
}

// resolveImage returns a local path for a file that may be either a local path
// or a remote URL, downloading and caching the latter.
func resolveImage(file string) (string, error) {
	if isURLImage(file) {
		return downloadImage(file)
	}
	return file, nil
}

func imagesCacheDir() (string, error) {
	dir := filepath.Join(config.Get().DataPath, "images")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("MicroVMDriver: failed to create images cache directory: %w", err)
	}
	return dir, nil
}

func overlaysDir(loadName string) (string, error) {
	dir := filepath.Join(config.Get().DataPath, "overlays", loadName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("MicroVMDriver: failed to create overlays directory: %w", err)
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
		slog.Info("fc_images.downloadImage", "msg", "using cached image", "url", rawURL, "path", cachedPath)
		return cachedPath, nil
	}

	slog.Info("fc_images.downloadImage", "msg", "downloading image", "url", rawURL)

	tmpFile, err := os.CreateTemp(cacheDir, ".tmp.*")
	if err != nil {
		return "", fmt.Errorf("MicroVMDriver: failed to create temp file for download: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		tmpFile.Close()
		os.Remove(tmpPath)
	}()

	resp, err := http.Get(rawURL)
	if err != nil {
		return "", fmt.Errorf("MicroVMDriver: failed to download image from %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("MicroVMDriver: failed to download image from %s: HTTP %d", rawURL, resp.StatusCode)
	}

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		return "", fmt.Errorf("MicroVMDriver: failed to write downloaded image: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("MicroVMDriver: failed to close downloaded image: %w", err)
	}

	if err := os.Rename(tmpPath, cachedPath); err != nil {
		// another process may have placed the file already
		if _, statErr := os.Stat(cachedPath); statErr == nil {
			return cachedPath, nil
		}
		return "", fmt.Errorf("MicroVMDriver: failed to move downloaded image to cache: %w", err)
	}

	slog.Info("fc_images.downloadImage", "msg", "image downloaded", "url", rawURL, "path", cachedPath)
	return cachedPath, nil
}

func createDrives(drives []FCDrive, loadName string) (map[int]OverlayImage, error) {
	overlays := make(map[int]OverlayImage)
	for i := range drives {
		if drives[i].File == "" {
			return nil, fmt.Errorf("MicroVMDriver: drive %d has no file", i)
		}

		imagePath, err := resolveImage(drives[i].File)
		if err != nil {
			return nil, err
		}

		overlayImage, err := CreateOverlay(loadName, imagePath)
		if err != nil {
			return nil, err
		}

		if drives[i].Resize != "" {
			if err := resizeOverlay(overlayImage.FilePath, drives[i].Resize); err != nil {
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

// resizeOverlay grows a raw image in place. Firecracker only accepts raw block
// images, so there is no format to convert and a truncate is enough; growing
// the filesystem inside is still the guest's job (cloud-init's growpart does
// it).
func resizeOverlay(filePath, size string) error {
	capacity, err := parseDriveSize(size)
	if err != nil {
		return fmt.Errorf("MicroVMDriver: invalid resize for %s: %w", filePath, err)
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("MicroVMDriver: failed to stat overlay %s: %w", filePath, err)
	}
	if uint64(info.Size()) >= capacity {
		return nil
	}

	slog.Info("fc_images.resizeOverlay", "msg", "resizing overlay", "path", filePath, "size", size, "bytes", capacity)
	if err := os.Truncate(filePath, int64(capacity)); err != nil {
		return fmt.Errorf("MicroVMDriver: failed to resize overlay %s: %w", filePath, err)
	}
	return nil
}

func cleanupOverlays(loadName string) {
	dir := filepath.Join(config.Get().DataPath, "overlays", loadName)
	if err := os.RemoveAll(dir); err != nil {
		slog.Warn("fc_images.cleanupOverlays", "msg", "failed to clean up overlays", "load", loadName, "error", err)
	}
}

func stringToSliceHook(from reflect.Type, to reflect.Type, data any) (any, error) {
	if from.Kind() == reflect.String && to == reflect.TypeFor[[]string]() {
		return []string{data.(string)}, nil
	}
	return data, nil
}

func runtimeDir(loadName string) (string, error) {
	dir := filepath.Join(config.Get().DataPath, "microvm", loadName)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return "", fmt.Errorf("MicroVMDriver: failed to create runtime directory: %w", err)
	}
	return dir, nil
}

func defaultAPISocket(loadName string) (string, error) {
	dir, err := runtimeDir(loadName)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "api.socket"), nil
}

func rootDriveIndex(drives []FCDrive) int {
	for i := range drives {
		if drives[i].Root {
			return i
		}
	}
	return -1
}

// rootDevice maps the root drive to its guest device name. Firecracker exposes
// drives as virtio-blk in attach order, so index 0 is /dev/vda.
func rootDevice(drives []FCDrive) (string, bool) {
	i := rootDriveIndex(drives)
	if i < 0 || i > 25 {
		return "", false
	}
	return fmt.Sprintf("/dev/vd%c", 'a'+i), true
}

func driveID(d FCDrive, index int) string {
	if d.ID != "" {
		return d.ID
	}
	if d.Root {
		return "rootfs"
	}
	return fmt.Sprintf("drive%d", index)
}

func netdevID(nd FCNetdev, index int) string {
	if nd.ID != "" {
		return nd.ID
	}
	return fmt.Sprintf("eth%d", index)
}
