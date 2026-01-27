package loads

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bitomia/realm/common"
	"github.com/google/uuid"
)

type DiskImage struct {
	File      string `json:"file"`                // Path to disk image
	Format    string `json:"format,omitempty"`    // Image format (qcow2, raw, vmdk, etc.)
	Interface string `json:"interface,omitempty"` // Interface type (virtio, ide, scsi)
	ReadOnly  bool   `json:"readonly,omitempty"`  // Mount as read-only
	Cache     string `json:"cache,omitempty"`     // Cache mode (none, writethrough, writeback)
}

type USBDevice struct {
	VendorID  string `json:"vendor_id,omitempty"`  // USB vendor ID (e.g., "0x1234")
	ProductID string `json:"product_id,omitempty"` // USB product ID (e.g., "0x5678")
	HostBus   string `json:"host_bus,omitempty"`   // Host bus number
	HostAddr  string `json:"host_addr,omitempty"`  // Host address
}

type CloudInitConfig struct {
	Enabled       bool              `json:"enabled"`                  // Enable cloud-init
	UserData      string            `json:"user_data,omitempty"`      // Path to user-data file
	MetaData      string            `json:"meta_data,omitempty"`      // Path to meta-data file
	NetworkConfig string            `json:"network_config,omitempty"` // Path to network-config file
	ISOPath       string            `json:"iso_path,omitempty"`       // Path to generated cloud-init ISO
	Inline        map[string]string `json:"inline,omitempty"`         // Inline cloud-init data
}

type BootConfig struct {
	Firmware string `json:"firmware,omitempty"` // Firmware type: "bios" (default), "uefi", or path to custom firmware
	Order    string `json:"order,omitempty"`    // Boot order (e.g., "cdn" for cd, disk, network)
	Menu     bool   `json:"menu,omitempty"`     // Enable boot menu
}

type QemuMetadata struct {
	VMName       string `json:"vm_name"`
	PID          int    `json:"pid"`
	PIDFile      string `json:"pid_file"`
	StdoutLog    string `json:"stdout_log"`
	StderrLog    string `json:"stderr_log"`
	Image        string `json:"image"`
	CPUs         int    `json:"cpus"`
	Memory       string `json:"memory"`
	WorkingDir   string `json:"working_dir"`
	SerialLog    string `json:"serial_log,omitempty"`
	MonitorPort  int    `json:"monitor_port,omitempty"`
	CloudInitISO string `json:"cloud_init_iso,omitempty"`
	CloudInitDir string `json:"cloud_init_dir,omitempty"`
	SnapshotMode bool   `json:"snapshot_mode,omitempty"`
}

type QemuDriver struct {
	Image           string            `json:"image"`                      // Path to the primary disk image (qcow2, raw, etc.)
	CPUs            int               `json:"cpus"`                       // Number of virtual CPUs
	Memory          string            `json:"memory"`                     // Memory size (e.g., "2G", "512M")
	MachineType     string            `json:"machine_type"`               // QEMU machine type (e.g., "pc", "q35")
	Accelerator     string            `json:"accelerator"`                // Acceleration type (e.g., "kvm", "tcg")
	NetDevice       string            `json:"net_device"`                 // Network device type (e.g., "user", "bridge", "tap")
	NetOptions      map[string]string `json:"net_options"`                // Additional network options
	ExtraArgs       []string          `json:"extra_args"`                 // Additional QEMU arguments
	QemuBinary      string            `json:"qemu_binary"`                // Path to QEMU binary (default: qemu-system-x86_64)
	VNCDisplay      string            `json:"vnc_display"`                // VNC display number (e.g., ":0")
	NoGraphic       bool              `json:"no_graphic"`                 // Disable graphical output
	SerialLog       string            `json:"serial_log"`                 // Path to serial console log file
	MonitorPort     int               `json:"monitor_port"`               // QMP monitor port for control
	WorkingDir      string            `json:"working_dir"`                // Working directory for QEMU process
	AdditionalDisks []DiskImage       `json:"additional_disks,omitempty"` // Additional disk images to attach
	USBDevices      []USBDevice       `json:"usb_devices,omitempty"`      // USB devices to pass through
	CloudInit       *CloudInitConfig  `json:"cloud_init,omitempty"`       // Cloud-init configuration
	Boot            *BootConfig       `json:"boot,omitempty"`             // Boot configuration
	Snapshot        bool              `json:"snapshot,omitempty"`         // Run in snapshot mode (changes not saved)
	CPUFlags        []string          `json:"cpu_flags,omitempty"`        // Additional CPU flags (e.g., "+vmx", "+svm")
	NUMA            bool              `json:"numa,omitempty"`             // Enable NUMA
	Hugepages       bool              `json:"hugepages,omitempty"`        // Use hugepages for memory
	RTC             string            `json:"rtc,omitempty"`              // RTC configuration (e.g., "base=utc,clock=host")
}

func (d QemuDriver) GetLoadDriverID() common.LoadDriverID {
	return common.LoadDriverID("qemu")
}

func (d QemuDriver) DriverInfo() common.LoadDriverInfo {
	return common.LoadDriverInfo{
		ID: d.GetLoadDriverID(),
		New: func(config any) (common.LoadDriver, error) {
			configData, err := json.Marshal(config)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal config: %w", err)
			}

			var driver QemuDriver
			if err := json.Unmarshal(configData, &driver); err != nil {
				return nil, fmt.Errorf("failed to unmarshal qemu config: %w", err)
			}

			return driver, nil
		},
	}
}

func (d QemuDriver) GetDriverConfig() common.LoadDriverConfig {
	return common.LoadDriverConfig{
		Driver:       d.GetLoadDriverID(),
		DriverConfig: d,
	}
}

func (d QemuDriver) Verify() error {
	if d.Image == "" {
		return fmt.Errorf("image path is required")
	}

	// Check if image file exists
	if _, err := os.Stat(d.Image); err != nil {
		return fmt.Errorf("image file not found: %s", d.Image)
	}

	// Set defaults
	if d.CPUs <= 0 {
		d.CPUs = 1
	}

	if d.Memory == "" {
		d.Memory = "1G"
	}

	if d.QemuBinary == "" {
		d.QemuBinary = "qemu-system-x86_64"
	}

	// Verify QEMU binary exists
	if _, err := exec.LookPath(d.QemuBinary); err != nil {
		return fmt.Errorf("qemu binary '%s' not found in PATH", d.QemuBinary)
	}

	if d.MachineType == "" {
		d.MachineType = "pc"
	}

	if d.Accelerator == "" {
		d.Accelerator = "kvm"
	}

	if d.NetDevice == "" {
		d.NetDevice = "user"
	}

	// Verify additional disks exist
	for i, disk := range d.AdditionalDisks {
		if _, err := os.Stat(disk.File); err != nil {
			return fmt.Errorf("additional disk %d not found: %s", i, disk.File)
		}
	}

	// Validate cloud-init configuration
	if d.CloudInit != nil && d.CloudInit.Enabled {
		if d.CloudInit.UserData != "" {
			if _, err := os.Stat(d.CloudInit.UserData); err != nil {
				return fmt.Errorf("cloud-init user-data file not found: %s", d.CloudInit.UserData)
			}
		}
		if d.CloudInit.MetaData != "" {
			if _, err := os.Stat(d.CloudInit.MetaData); err != nil {
				return fmt.Errorf("cloud-init meta-data file not found: %s", d.CloudInit.MetaData)
			}
		}
	}

	// Validate boot firmware
	if d.Boot != nil && d.Boot.Firmware != "" && d.Boot.Firmware != "uefi" && d.Boot.Firmware != "bios" {
		if _, err := os.Stat(d.Boot.Firmware); err != nil {
			return fmt.Errorf("boot firmware file not found: %s", d.Boot.Firmware)
		}
	}

	// Verify hugepages if enabled
	if d.Hugepages {
		if _, err := os.Stat("/dev/hugepages"); err != nil {
			slog.Warn("Hugepages requested but /dev/hugepages not found")
		}
	}

	return nil
}

func (d QemuDriver) generateCloudInitISO(vmName string) (string, error) {
	if d.CloudInit == nil || !d.CloudInit.Enabled {
		return "", nil
	}

	// Create temporary directory for cloud-init files
	tmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("realm-cloudinit-%s", vmName))
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create cloud-init temp dir: %w", err)
	}

	// Copy or create cloud-init files
	userDataPath := filepath.Join(tmpDir, "user-data")
	metaDataPath := filepath.Join(tmpDir, "meta-data")

	if d.CloudInit.UserData != "" {
		data, err := os.ReadFile(d.CloudInit.UserData)
		if err != nil {
			return "", fmt.Errorf("failed to read user-data: %w", err)
		}
		if err := os.WriteFile(userDataPath, data, 0644); err != nil {
			return "", fmt.Errorf("failed to write user-data: %w", err)
		}
	} else if d.CloudInit.Inline != nil {
		if userData, ok := d.CloudInit.Inline["user-data"]; ok {
			if err := os.WriteFile(userDataPath, []byte(userData), 0644); err != nil {
				return "", fmt.Errorf("failed to write inline user-data: %w", err)
			}
		}
	}

	if d.CloudInit.MetaData != "" {
		data, err := os.ReadFile(d.CloudInit.MetaData)
		if err != nil {
			return "", fmt.Errorf("failed to read meta-data: %w", err)
		}
		if err := os.WriteFile(metaDataPath, data, 0644); err != nil {
			return "", fmt.Errorf("failed to write meta-data: %w", err)
		}
	} else {
		// Create minimal meta-data
		metaData := fmt.Sprintf("instance-id: %s\nlocal-hostname: %s\n", vmName, vmName)
		if err := os.WriteFile(metaDataPath, []byte(metaData), 0644); err != nil {
			return "", fmt.Errorf("failed to write meta-data: %w", err)
		}
	}

	// Generate ISO using genisoimage or mkisofs
	isoPath := filepath.Join(tmpDir, "cloud-init.iso")
	var cmd *exec.Cmd

	if _, err := exec.LookPath("genisoimage"); err == nil {
		cmd = exec.Command("genisoimage", "-output", isoPath, "-volid", "cidata", "-joliet", "-rock", userDataPath, metaDataPath)
	} else if _, err := exec.LookPath("mkisofs"); err == nil {
		cmd = exec.Command("mkisofs", "-output", isoPath, "-volid", "cidata", "-joliet", "-rock", userDataPath, metaDataPath)
	} else {
		return "", fmt.Errorf("neither genisoimage nor mkisofs found in PATH")
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to generate cloud-init ISO: %w, output: %s", err, string(output))
	}

	slog.Debug("Generated cloud-init ISO", "path", isoPath)
	return isoPath, nil
}

func (d QemuDriver) PlanAndRegister(repository common.DeploymentsRepository, loadName string) (common.DeploymentID, error) {
	slog.Debug("Planning QEMU VM load", "loadName", loadName)

	if err := d.Verify(); err != nil {
		return uuid.Nil, fmt.Errorf("verification failed: %w", err)
	}

	// Check if KVM is available when using KVM accelerator
	if d.Accelerator == "kvm" {
		if _, err := os.Stat("/dev/kvm"); err != nil {
			slog.Warn("KVM device not found, will use TCG acceleration instead")
		}
	}

	// Verify cloud-init tools if needed
	if d.CloudInit != nil && d.CloudInit.Enabled && d.CloudInit.ISOPath == "" {
		if _, err := exec.LookPath("genisoimage"); err != nil {
			if _, err := exec.LookPath("mkisofs"); err != nil {
				return uuid.Nil, fmt.Errorf("cloud-init enabled but neither genisoimage nor mkisofs found in PATH")
			}
		}
	}

	// Create deployment in "planned" state
	did, err := repository.Create(loadName, d, common.DeploymentStatePlanned, QemuMetadata{})
	if err != nil {
		slog.Error("QemuDriver.PlanAndRegister", "msg", "failed to create deployment", "error", err)
		return uuid.Nil, err
	}

	return did, nil
}

func (d QemuDriver) buildQemuArgs(vmName string) []string {
	args := []string{
		"-name", vmName,
		"-machine", d.MachineType,
		"-accel", d.Accelerator,
	}

	// CPU configuration
	cpuArgs := "host"
	if len(d.CPUFlags) > 0 {
		cpuArgs += "," + strings.Join(d.CPUFlags, ",")
	}
	args = append(args, "-cpu", cpuArgs)
	args = append(args, "-smp", strconv.Itoa(d.CPUs))

	// Memory configuration
	memArgs := []string{"-m", d.Memory}
	if d.Hugepages {
		memArgs = append(memArgs, "-mem-prealloc", "-mem-path", "/dev/hugepages")
	}
	args = append(args, memArgs...)

	// NUMA configuration
	if d.NUMA && d.CPUs > 1 {
		args = append(args, "-numa", "node,cpus=0-"+strconv.Itoa(d.CPUs-1))
	}

	// Boot configuration
	if d.Boot != nil {
		if d.Boot.Firmware == "uefi" || strings.HasPrefix(d.Boot.Firmware, "/") {
			firmwarePath := d.Boot.Firmware
			if firmwarePath == "uefi" {
				firmwarePath = "/usr/share/OVMF/OVMF_CODE.fd"
			}
			args = append(args, "-bios", firmwarePath)
		}
		if d.Boot.Order != "" {
			bootOrder := fmt.Sprintf("order=%s", d.Boot.Order)
			if d.Boot.Menu {
				bootOrder += ",menu=on"
			}
			args = append(args, "-boot", bootOrder)
		}
	}

	// Primary disk
	driveArgs := fmt.Sprintf("file=%s,if=virtio,format=qcow2", d.Image)
	if d.Snapshot {
		driveArgs += ",snapshot=on"
	}
	args = append(args, "-drive", driveArgs)

	// Additional disks
	for i, disk := range d.AdditionalDisks {
		diskFormat := disk.Format
		if diskFormat == "" {
			diskFormat = "qcow2"
		}
		diskIf := disk.Interface
		if diskIf == "" {
			diskIf = "virtio"
		}
		diskArgs := fmt.Sprintf("file=%s,if=%s,format=%s,index=%d", disk.File, diskIf, diskFormat, i+1)
		if disk.ReadOnly {
			diskArgs += ",readonly=on"
		}
		if disk.Cache != "" {
			diskArgs += fmt.Sprintf(",cache=%s", disk.Cache)
		}
		args = append(args, "-drive", diskArgs)
	}

	// Cloud-init support
	if d.CloudInit != nil && d.CloudInit.Enabled {
		if d.CloudInit.ISOPath != "" {
			args = append(args, "-drive", fmt.Sprintf("file=%s,if=virtio,format=raw,media=cdrom", d.CloudInit.ISOPath))
		}
	}

	// Network configuration
	netArgs := fmt.Sprintf("%s", d.NetDevice)
	if len(d.NetOptions) > 0 {
		for key, value := range d.NetOptions {
			netArgs += fmt.Sprintf(",%s=%s", key, value)
		}
	}
	args = append(args, "-netdev", netArgs, "-device", "virtio-net-pci,netdev=net0")

	// USB controller and devices
	if len(d.USBDevices) > 0 {
		args = append(args, "-usb")
		for _, usb := range d.USBDevices {
			if usb.VendorID != "" && usb.ProductID != "" {
				args = append(args, "-device", fmt.Sprintf("usb-host,vendorid=%s,productid=%s", usb.VendorID, usb.ProductID))
			} else if usb.HostBus != "" && usb.HostAddr != "" {
				args = append(args, "-device", fmt.Sprintf("usb-host,hostbus=%s,hostaddr=%s", usb.HostBus, usb.HostAddr))
			}
		}
	}

	// Display configuration
	if d.NoGraphic {
		args = append(args, "-nographic")
	} else if d.VNCDisplay != "" {
		args = append(args, "-vnc", d.VNCDisplay)
	} else {
		args = append(args, "-display", "none")
	}

	// Serial console logging
	if d.SerialLog != "" {
		args = append(args, "-serial", fmt.Sprintf("file:%s", d.SerialLog))
	} else {
		args = append(args, "-serial", "null")
	}

	// RTC configuration
	if d.RTC != "" {
		args = append(args, "-rtc", d.RTC)
	} else {
		args = append(args, "-rtc", "base=utc,clock=host")
	}

	// QMP monitor for control
	if d.MonitorPort > 0 {
		args = append(args, "-qmp", fmt.Sprintf("tcp:127.0.0.1:%d,server,nowait", d.MonitorPort))
	}

	// Daemonize QEMU
	args = append(args, "-daemonize")

	// PID file for tracking
	pidFile := filepath.Join(os.TempDir(), fmt.Sprintf("%s.pid", vmName))
	args = append(args, "-pidfile", pidFile)

	// Additional user-specified arguments
	if len(d.ExtraArgs) > 0 {
		args = append(args, d.ExtraArgs...)
	}

	return args
}

func (d QemuDriver) StartDeployment(repository common.DeploymentsRepository, deployment common.Deployment) error {
	// Verify deployment is in "planned" state
	if deployment.State != common.DeploymentStatePlanned {
		return fmt.Errorf("deployment %s is not in planned state", deployment.ID)
	}

	loadName := deployment.LoadName
	slog.Info("QemuDriver.StartDeployment", "msg", "starting QEMU VM", "deployment", deployment.ID, "loadName", loadName)

	// Generate unique VM name
	vmName := fmt.Sprintf("%s-%s", loadName, uuid.New().String()[:8])

	// Generate cloud-init ISO if needed
	if d.CloudInit != nil && d.CloudInit.Enabled && d.CloudInit.ISOPath == "" {
		isoPath, err := d.generateCloudInitISO(vmName)
		if err != nil {
			return fmt.Errorf("failed to generate cloud-init ISO: %w", err)
		}
		d.CloudInit.ISOPath = isoPath
	}

	// Prepare working directory
	workingDir := d.WorkingDir
	if workingDir == "" {
		workingDir = filepath.Dir(d.Image)
	}

	// Build QEMU command
	args := d.buildQemuArgs(vmName)
	cmd := exec.Command(d.QemuBinary, args...)
	cmd.Dir = workingDir

	// Setup log files
	logDir := filepath.Join(os.TempDir(), "realm-qemu-logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	stdoutLog := filepath.Join(logDir, fmt.Sprintf("%s-stdout.log", vmName))
	stderrLog := filepath.Join(logDir, fmt.Sprintf("%s-stderr.log", vmName))

	stdoutFile, err := os.Create(stdoutLog)
	if err != nil {
		return fmt.Errorf("failed to create stdout log: %w", err)
	}
	defer stdoutFile.Close()

	stderrFile, err := os.Create(stderrLog)
	if err != nil {
		return fmt.Errorf("failed to create stderr log: %w", err)
	}
	defer stderrFile.Close()

	cmd.Stdout = stdoutFile
	cmd.Stderr = stderrFile

	// Start QEMU
	slog.Debug("Executing QEMU command", "binary", d.QemuBinary, "args", strings.Join(args, " "))
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start QEMU: %w", err)
	}

	// Wait a moment for QEMU to daemonize
	time.Sleep(500 * time.Millisecond)

	// Read PID from pidfile
	pidFile := filepath.Join(os.TempDir(), fmt.Sprintf("%s.pid", vmName))
	pidData, err := os.ReadFile(pidFile)
	if err != nil {
		return fmt.Errorf("failed to read PID file: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidData)))
	if err != nil {
		return fmt.Errorf("failed to parse PID: %w", err)
	}

	// Store deployment metadata
	metadata := QemuMetadata{
		VMName:     vmName,
		PID:        pid,
		PIDFile:    pidFile,
		StdoutLog:  stdoutLog,
		StderrLog:  stderrLog,
		Image:      d.Image,
		CPUs:       d.CPUs,
		Memory:     d.Memory,
		WorkingDir: workingDir,
	}

	if d.SerialLog != "" {
		metadata.SerialLog = d.SerialLog
	}
	if d.MonitorPort > 0 {
		metadata.MonitorPort = d.MonitorPort
	}
	if d.CloudInit != nil && d.CloudInit.ISOPath != "" {
		metadata.CloudInitISO = d.CloudInit.ISOPath
		metadata.CloudInitDir = filepath.Dir(d.CloudInit.ISOPath)
	}
	if d.Snapshot {
		metadata.SnapshotMode = true
	}

	// Update metadata
	if err := common.UpdateMetadata(repository, deployment.ID, func(meta *QemuMetadata) error {
		*meta = metadata
		return nil
	}); err != nil {
		// Try to stop the VM if we can't save the metadata
		d.killVM(pid, pidFile)
		return fmt.Errorf("failed to update deployment metadata: %w", err)
	}

	// Update deployment state to "running"
	if err := repository.UpdateState(deployment.ID, common.DeploymentStateRunning); err != nil {
		slog.Error("QemuDriver.StartDeployment", "msg", "failed to update deployment state", "error", err)
		// Try to stop the VM
		d.killVM(pid, pidFile)
		return err
	}

	slog.Info("QEMU VM started successfully", "vmName", vmName, "pid", pid)
	return nil
}

func (d QemuDriver) killVM(pid int, pidFile string) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process: %w", err)
	}

	// Try graceful shutdown first (SIGTERM)
	slog.Debug("Sending SIGTERM to QEMU process", "pid", pid)
	if err := process.Signal(os.Interrupt); err != nil {
		slog.Warn("Failed to send SIGTERM", "error", err)
	}

	// Wait up to 10 seconds for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	done := make(chan bool)
	go func() {
		process.Wait()
		done <- true
	}()

	select {
	case <-done:
		slog.Debug("QEMU process terminated gracefully")
	case <-ctx.Done():
		// Force kill if graceful shutdown times out
		slog.Warn("QEMU process didn't terminate gracefully, force killing")
		if err := process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process: %w", err)
		}
	}

	// Clean up PID file
	if pidFile != "" {
		os.Remove(pidFile)
	}

	return nil
}

// sendQMPCommand sends a command to the QMP monitor and returns the response
func (d *QemuDriver) sendQMPCommand(monitorPort int, command map[string]any) (map[string]any, error) {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", monitorPort), 3*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to QMP: %w", err)
	}
	defer conn.Close()

	// Read QMP greeting
	reader := bufio.NewReader(conn)
	_, err = reader.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read QMP greeting: %w", err)
	}

	// Send qmp_capabilities to enter command mode
	capabilitiesCmd := map[string]any{"execute": "qmp_capabilities"}
	capabilitiesJSON, _ := json.Marshal(capabilitiesCmd)
	if _, err := conn.Write(append(capabilitiesJSON, '\n')); err != nil {
		return nil, fmt.Errorf("failed to send qmp_capabilities: %w", err)
	}
	// Read response
	_, err = reader.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read capabilities response: %w", err)
	}

	// Send actual command
	cmdJSON, err := json.Marshal(command)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal command: %w", err)
	}
	if _, err := conn.Write(append(cmdJSON, '\n')); err != nil {
		return nil, fmt.Errorf("failed to send command: %w", err)
	}

	// Read response
	responseBytes, err := reader.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var response map[string]any
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response, nil
}

func (d *QemuDriver) gracefulShutdownQMP(monitorPort int) error {
	cmd := map[string]any{"execute": "system_powerdown"}
	_, err := d.sendQMPCommand(monitorPort, cmd)
	return err
}

func (d QemuDriver) StopDeployment(repository common.DeploymentsRepository, deployment common.Deployment) error {
	// Verify deployment is in "running" state
	if deployment.State != common.DeploymentStateRunning {
		return fmt.Errorf("deployment %s is not in running state", deployment.ID)
	}

	slog.Info("QemuDriver.StopDeployment", "msg", "stopping QEMU VM", "deployment", deployment.ID)

	// Unmarshal metadata
	var metadata QemuMetadata
	if tmp, err := json.Marshal(deployment.Metadata); err != nil {
		slog.Error("QemuDriver.StopDeployment", "error", "error on retrieving metadata", "deployment", deployment.ID)
		return err
	} else {
		json.Unmarshal(tmp, &metadata)
	}

	vmName := metadata.VMName
	pid := metadata.PID
	pidFile := metadata.PIDFile
	cloudInitDir := metadata.CloudInitDir

	slog.Info("QemuDriver.StopDeployment", "msg", "retrieved VM metadata", "deployment", deployment.ID, "vmName", vmName)

	// Try graceful shutdown via QMP if monitor is available
	if metadata.MonitorPort > 0 {
		slog.Debug("Attempting graceful shutdown via QMP")
		if err := d.gracefulShutdownQMP(metadata.MonitorPort); err != nil {
			slog.Warn("QMP shutdown failed, falling back to signal", "error", err)
		} else {
			// Wait up to 15 seconds for graceful shutdown
			process, _ := os.FindProcess(pid)
			done := make(chan bool)
			go func() {
				process.Wait()
				done <- true
			}()

			select {
			case <-done:
				slog.Debug("VM shut down gracefully via QMP")
				goto cleanup
			case <-time.After(15 * time.Second):
				slog.Warn("QMP shutdown timeout, forcing kill")
			}
		}
	}

	// Kill the VM via signals
	if err := d.killVM(pid, pidFile); err != nil {
		slog.Warn("Error killing VM", "error", err)
	}

cleanup:
	// Clean up cloud-init files
	if cloudInitDir != "" {
		slog.Debug("Cleaning up cloud-init directory", "path", cloudInitDir)
		if err := os.RemoveAll(cloudInitDir); err != nil {
			slog.Warn("Failed to clean up cloud-init directory", "error", err)
		}
	}

	// Clear metadata
	if err := common.UpdateMetadata(repository, deployment.ID, func(meta *QemuMetadata) error {
		*meta = QemuMetadata{}
		return nil
	}); err != nil {
		return err
	}

	// Update deployment state back to "planned"
	if err := repository.UpdateState(deployment.ID, common.DeploymentStatePlanned); err != nil {
		slog.Error("QemuDriver.StopDeployment", "msg", "failed to update deployment state", "deploymentID", deployment.ID, "error", err)
		return fmt.Errorf("failed to update deployment state: %w", err)
	}

	slog.Info("QEMU VM stopped successfully", "vmName", vmName)
	return nil
}

func (d QemuDriver) UnplanDeployment(repository common.DeploymentsRepository, deployment common.Deployment) error {
	// Verify deployment is in "planned" state
	if deployment.State != common.DeploymentStatePlanned {
		return fmt.Errorf("deployment %s is not in planned state", deployment.ID)
	}

	slog.Info("QemuDriver.UnplanDeployment", "msg", "removing planned deployment", "deployment", deployment.ID)

	// For QEMU, there's nothing to clean up at unplan time
	// (no VM created yet, just verification was done)
	if err := repository.DeleteDeployment(deployment.ID); err != nil {
		slog.Error("QemuDriver.UnplanDeployment", "msg", "failed to delete deployment", "deploymentID", deployment.ID, "error", err)
		return fmt.Errorf("failed to delete deployment: %w", err)
	}

	return nil
}

func (d QemuDriver) MarshalJSON() ([]byte, error) {
	type Alias QemuDriver
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(&d),
	})
}

func (d QemuDriver) UnmarshalJSON(data []byte) error {
	type Alias QemuDriver
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(&d),
	}
	return json.Unmarshal(data, &aux)
}

func (q QemuDriver) ReadStdout(repository common.DeploymentsRepository, deployment common.Deployment, w io.Writer) error {
	// TODO
	return nil
}

func (q QemuDriver) ReadStderr(repository common.DeploymentsRepository, deployment common.Deployment, w io.Writer) error {
	// TODO
	return nil
}
