//go:build linux

package microvm

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/google/uuid"
	"github.com/vishvananda/netlink"

	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/internal"
)

const MicroVMDriverID common.LoadDriverID = "microvm"

type MicroVMConfig struct {
	Binary   string     `json:"binary,omitempty"`
	Kernel   string     `json:"kernel,omitempty"`
	Initrd   string     `json:"initrd,omitempty"`
	BootArgs string     `json:"boot_args,omitempty"`
	Memory   int        `json:"memory,omitempty"`
	VCPUs    int        `json:"vcpus,omitempty"`
	SMT      bool       `json:"smt,omitempty"`
	Drives   []FCDrive  `json:"drives,omitempty"`
	Netdev   []FCNetdev `json:"netdev,omitempty"`
	Balloon  *FCBalloon `json:"balloon,omitempty"`
}

type MicroVMDriver struct {
	config MicroVMConfig
}

type MicroVMEntryMetadata struct {
	VMName     string               `json:"vm_name"`
	PID        int                  `json:"pid"`
	APISocket  string               `json:"api_socket"`
	KernelPath string               `json:"kernel_path"`
	InitrdPath string               `json:"initrd_path,omitempty"`
	BootPlan   BootPlan             `json:"boot_plan"`
	Drives     map[int]OverlayImage `json:"overlay_drives"`
	StdoutPath string               `json:"stdout_path,omitempty"`
	StderrPath string               `json:"stderr_path,omitempty"`
}

func NewMicroVMDriver(c any) (common.LoadDriver, error) {
	var config MicroVMConfig
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		TagName:    "json",
		Result:     &config,
		DecodeHook: stringToSliceHook,
	})
	if err != nil {
		return nil, err
	}
	if err := decoder.Decode(c); err != nil {
		return nil, err
	}

	if config.Kernel == "" {
		return nil, fmt.Errorf("MicroVMDriver.PowerOn: kernel is required")
	}

	if config.Binary == "" {
		config.Binary = "firecracker"
	}

	binary, err := exec.LookPath(config.Binary)
	if err != nil {
		return nil, fmt.Errorf("MicroVMDriver: %q is not available: %w", config.Binary, err)
	}
	config.Binary = binary

	driver := &MicroVMDriver{
		config: config,
	}

	return driver, nil
}

func (m *MicroVMDriver) ID() common.LoadDriverID {
	return MicroVMDriverID
}

func (m *MicroVMDriver) Info() common.LoadDriverInfo {
	return common.LoadDriverInfo{
		ID:  MicroVMDriverID,
		New: NewMicroVMDriver,
	}
}

func (m *MicroVMDriver) Provision(nodeDriver common.NodeDriver, repository common.DeploymentsRepository, loadName string) (common.DeploymentID, error) {
	slog.Info("MicroVMDriver.Provision", "msg", "provisioning deployment", "load_name", loadName)

	runtimeDir, err := runtimeDir(loadName)
	if err != nil {
		return uuid.Nil, fmt.Errorf("MicroVMDriver.Provision: failed to obtain runtime dir: %w", err)
	}

	kernelPath, err := resolveImage(m.config.Kernel)
	if err != nil {
		return uuid.Nil, fmt.Errorf("MicroVMDriver.Provision: failed to resolve kernel: %w", err)
	}

	initrdPath := ""
	if m.config.Initrd != "" {
		if initrdPath, err = resolveImage(m.config.Initrd); err != nil {
			return uuid.Nil, fmt.Errorf("MicroVMDriver.Provision: failed to resolve initrd: %w", err)
		}
	}

	overlayDrives, err := createDrives(m.config.Drives, loadName)
	if err != nil {
		return uuid.Nil, fmt.Errorf("MicroVMDriver.PowerOn: failed to resolve drive images: %w", err)
	}

	bootArgs := m.buildBootArgs()
	timestamp := time.Now().Format(time.RFC3339)
	metadata := MicroVMEntryMetadata{
		VMName:     loadName,
		PID:        -1,
		APISocket:  "",
		KernelPath: kernelPath,
		InitrdPath: initrdPath,
		BootPlan: BootPlan{
			KernelPath: kernelPath,
			InitrdPath: initrdPath,
			BootArgs:   bootArgs,
			Drives:     overlayDrives,
		},
		Drives:     overlayDrives,
		StdoutPath: filepath.Join(runtimeDir, timestamp, "stdout.log"),
		StderrPath: filepath.Join(runtimeDir, timestamp, "stderr.log"),
	}
	if err := os.MkdirAll(filepath.Dir(metadata.StdoutPath), 0750); err != nil {
		return uuid.Nil, fmt.Errorf("MicroVMDriver.Provision: failed to create stdout log directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(metadata.StderrPath), 0750); err != nil {
		return uuid.Nil, fmt.Errorf("MicroVMDriver.Provision: failed to create stderr log directory: %w", err)
	}

	did, err := repository.Create(loadName, m, common.DeploymentStatus{StatusCode: common.DeploymentStatusReady}, metadata)
	if err != nil {
		slog.Error("MicroVMDriver.Provision", "msg", "failed to create deployment", "error", err)
		return uuid.Nil, err
	}

	return did, nil
}

func (m *MicroVMDriver) Deprovision(repository common.DeploymentsRepository, deployment common.Deployment) error {
	slog.Info("MicroVMDriver.Deprovision", "msg", "deprovisioning deployment", "deployment", deployment.ID)

	if deployment.Status.StatusCode == common.DeploymentStatusError {
		metadata, err := getMicroVMMetadata(deployment)
		if err != nil {
			slog.Warn("MicroVMDriver.Deprovision", "error", "error on retrieving metadata", "deployment", deployment.ID)
			goto deprovision_deployment
		}

		if len(metadata.APISocket) > 0 && metadata.PID != -1 {
			if err := m.teardownVMM(metadata.APISocket, metadata.PID); err != nil {
				return err
			}
		}
	}

deprovision_deployment:
	if err := repository.DeleteDeployment(deployment.ID); err != nil {
		slog.Error("MicroVMDriver.Deprovision", "msg", "failed to delete deployment", "deploymentID", deployment.ID, "error", err)

		return repository.UpdateStatus(deployment.ID, common.DeploymentStatus{StatusCode: common.DeploymentStatusError, Reason: fmt.Sprintf("failed to delete deployment: %s", err.Error())})
	}

	runtimeDir, err := runtimeDir(deployment.LoadName)
	if err != nil {
		return fmt.Errorf("MicroVMDriver.Deprovision: failed to obtain runtime dir: %w", err)
	}
	if err := os.RemoveAll(runtimeDir); err != nil {
		return fmt.Errorf("MicroVMDriver.Deprovision: failed pruning runtime dir: %w", err)
	}

	return nil
}

func (m *MicroVMDriver) Start(repository common.DeploymentsRepository, deployment common.Deployment) error {
	slog.Info("MicroVMDriver.Start", "msg", "starting deployment", "deployment", deployment.ID)

	if status, err := m.UpdateStatus(repository, deployment); err != nil {
		return err
	} else {
		switch status.StatusCode {
		case common.DeploymentStatusRunning:
			slog.Warn("MicroVMDriver.Start", "msg", "deployment already running")
			return nil
		case common.DeploymentStatusError:
			slog.Error("MicroVMDriver.Start", "msg", "deployment error state", "error", err)
			return err
		}
	}

	socket, err := defaultAPISocket(deployment.LoadName)
	if err != nil {
		return repository.UpdateStatus(deployment.ID, common.DeploymentStatus{StatusCode: common.DeploymentStatusError, Reason: err.Error()})
	}

	metadata, err := getMicroVMMetadata(deployment)
	if err != nil {
		return repository.UpdateStatus(deployment.ID, common.DeploymentStatus{StatusCode: common.DeploymentStatusError, Reason: err.Error()})
	}

	if err := common.UpdateDeploymentMetadata(repository, deployment.ID, func(metadata *MicroVMEntryMetadata) error {
		metadata.APISocket = socket
		return nil
	}); err != nil {
		return common.SetDeploymentError(repository, deployment, "MicroVMDriver.Start", "deployment", deployment.ID, "error", fmt.Sprintf("Failed to update metadata: %v", err))
	}
	metadata.APISocket = socket

	pid, err := m.launch(metadata)
	if err != nil {
		cleanupOverlays(deployment.LoadName)
		return repository.UpdateStatus(deployment.ID, common.DeploymentStatus{StatusCode: common.DeploymentStatusError, Reason: err.Error()})
	}

	if err := common.UpdateDeploymentMetadata(repository, deployment.ID, func(metadata *MicroVMEntryMetadata) error {
		metadata.PID = pid
		return nil
	}); err != nil {
		return common.SetDeploymentError(repository, deployment, "MicroVMDriver.Start", "deployment", deployment.ID, "error", fmt.Sprintf("Failed to update metadata: %v", err))
	}

	slog.Info("FCDriver.PowerOn", "msg", "microVM started", "deployment", deployment.ID, "pid", pid)
	return nil
}

func (m *MicroVMDriver) Stop(repository common.DeploymentsRepository, deployment common.Deployment) error {
	slog.Info("MicroVMDriver.Stop", "msg", "stop deployment", "deployment", deployment.ID)

	if status, err := m.UpdateStatus(repository, deployment); err != nil {
		return err
	} else {
		switch status.StatusCode {
		case common.DeploymentStatusStopped:
			slog.Warn("MicroVMDriver.Stop", "msg", "deployment already stopped")
			return nil
		case common.DeploymentStatusError:
			slog.Error("MicroVMDriver.Stop", "msg", "deployment error state", "reason", status.Reason)
			return fmt.Errorf("MicroVMDriver.Stop: deployment %s is in error state: %s", deployment.ID, status.Reason)
		}
	}

	metadata, err := getMicroVMMetadata(deployment)
	if err != nil {
		return repository.UpdateStatus(deployment.ID, common.DeploymentStatus{StatusCode: common.DeploymentStatusError, Reason: err.Error()})
	}

	// ACPI power button. Teardown afterwards covers a guest that ignored it.
	if err := sendAction(metadata.APISocket, actionSendCtrlAltDel); err != nil {
		slog.Warn("MicroVMDriver.Stop", "msg", "graceful shutdown failed", "deployment", deployment.ID, "error", err)
	} else {
		waitVMMExit(metadata.PID, gracefulShutdownTimeout)
	}

	if err := m.teardownVMM(metadata.APISocket, metadata.PID); err != nil {
		return err
	}

	return m.clearRuntimeMetadata(repository, deployment, "MicroVMDriver.Stop")
}

func (m *MicroVMDriver) Kill(repository common.DeploymentsRepository, deployment common.Deployment) error {
	slog.Info("MicroVMDriver.Kill", "msg", "kill deployment", "deployment", deployment.ID)

	metadata, err := getMicroVMMetadata(deployment)
	if err != nil {
		return repository.UpdateStatus(deployment.ID, common.DeploymentStatus{StatusCode: common.DeploymentStatusError, Reason: err.Error()})
	}

	if metadata.PID == -1 {
		slog.Warn("MicroVMDriver.Kill", "msg", "deployment not running", "deployment", deployment.ID)
		return nil
	}

	if err := m.teardownVMM(metadata.APISocket, metadata.PID); err != nil {
		return err
	}

	return m.clearRuntimeMetadata(repository, deployment, "MicroVMDriver.Kill")
}

// clearRuntimeMetadata drops the PID and API socket of a VMM that is gone
func (m *MicroVMDriver) clearRuntimeMetadata(repository common.DeploymentsRepository, deployment common.Deployment, caller string) error {
	if err := common.UpdateDeploymentMetadata(repository, deployment.ID, func(metadata *MicroVMEntryMetadata) error {
		metadata.PID = -1
		metadata.APISocket = ""
		return nil
	}); err != nil {
		return common.SetDeploymentError(repository, deployment, caller, "deployment", deployment.ID, "error", fmt.Sprintf("Failed to update metadata: %v", err))
	}
	return nil
}

func (m *MicroVMDriver) UpdateStatus(r common.DeploymentsRepository, d common.Deployment) (common.DeploymentStatus, error) {
	slog.Info("MicroVMDriver.UpdateStatus", "msg", "Update deployment status", "deployment", d.ID)

	status := d.Status

	// Keep on error if it was on error status before
	if d.Status.StatusCode == common.DeploymentStatusError {
		return status, nil
	}

	metadata, err := getMicroVMMetadata(d)
	if err != nil {
		return common.DeploymentStatus{StatusCode: common.DeploymentStatusError, Reason: err.Error()}, nil
	}

	socket, err := defaultAPISocket(d.LoadName)
	if err != nil {
		status = common.DeploymentStatus{StatusCode: common.DeploymentStatusError, Reason: err.Error()}
	} else {
		socketExists := true
		if _, err := os.Stat(socket); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				socketExists = false
			} else {
				status = common.DeploymentStatus{StatusCode: common.DeploymentStatusError, Reason: err.Error()}
				goto finish_microvm_update_status
			}
		}

		if metadata.PID == -1 && !socketExists {
			status = common.DeploymentStatus{StatusCode: common.DeploymentStatusReady, Reason: ""}
			goto finish_microvm_update_status
		}
		if metadata.PID == -1 && socketExists {
			status = common.DeploymentStatus{StatusCode: common.DeploymentStatusStopped, Reason: ""}
			goto finish_microvm_update_status
		}

		isPIDValid := internal.PIDExists(metadata.PID)
		if !isPIDValid && socketExists {
			status = common.DeploymentStatus{StatusCode: common.DeploymentStatusStopped, Reason: ""}
			goto finish_microvm_update_status
		}
		if isPIDValid && !socketExists {
			status = common.DeploymentStatus{StatusCode: common.DeploymentStatusError, Reason: "valid PID but API socket not exists"}
			goto finish_microvm_update_status
		}
		if !isPIDValid && !socketExists {
			status = common.DeploymentStatus{StatusCode: common.DeploymentStatusError, Reason: "invalid PID and API socket not exists"}
			goto finish_microvm_update_status
		}

		info, err := instanceInfo(socket)
		if err != nil {
			status = common.DeploymentStatus{StatusCode: common.DeploymentStatusError, Reason: fmt.Sprintf("Cannot retrieve instance info: %s", err.Error())}
			goto finish_microvm_update_status
		}

		switch info.State {
		case "Running":
			status = common.DeploymentStatus{StatusCode: common.DeploymentStatusRunning, Reason: ""}
		case "Resumed":
			status = common.DeploymentStatus{StatusCode: common.DeploymentStatusRunning, Reason: ""}
		case "Paused":
			status = common.DeploymentStatus{StatusCode: common.DeploymentStatusStopped, Reason: ""}
		case "Not started":
			status = common.DeploymentStatus{StatusCode: common.DeploymentStatusReady, Reason: ""}
		default:
			status = common.DeploymentStatus{StatusCode: common.DeploymentStatusError, Reason: "unknown state"}
		}
	}

finish_microvm_update_status:
	return status, nil
}

func (m *MicroVMDriver) Config() common.LoadDriverConfig {
	return common.LoadDriverConfig{Driver: MicroVMDriverID, DriverConfig: m.config}
}

func (m *MicroVMDriver) StreamStdout(repository common.DeploymentsRepository, deployment common.Deployment, w io.Writer) error {
	metadata, err := getMicroVMMetadata(deployment)
	if err != nil {
		return err
	}

	if len(metadata.StdoutPath) == 0 {
		return fmt.Errorf("stdout path empty")
	}

	return common.TailFile(metadata.StdoutPath, w)
}

func (m *MicroVMDriver) StreamStderr(repository common.DeploymentsRepository, deployment common.Deployment, w io.Writer) error {
	metadata, err := getMicroVMMetadata(deployment)
	if err != nil {
		return err
	}

	if len(metadata.StderrPath) == 0 {
		return fmt.Errorf("stderr path empty")
	}

	return common.TailFile(metadata.StderrPath, w)
}

func (m *MicroVMDriver) ReadStdout(repository common.DeploymentsRepository, deployment common.Deployment, offset int64) ([]byte, int64, error) {
	metadata, err := getMicroVMMetadata(deployment)
	if err != nil {
		return nil, 0, err
	}

	if len(metadata.StdoutPath) == 0 {
		return nil, 0, fmt.Errorf("stdout path empty")
	}

	return common.ReadFileAt(metadata.StdoutPath, offset)
}

func (m *MicroVMDriver) ReadStderr(repository common.DeploymentsRepository, deployment common.Deployment, offset int64) ([]byte, int64, error) {
	metadata, err := getMicroVMMetadata(deployment)
	if err != nil {
		return nil, 0, err
	}

	if len(metadata.StderrPath) == 0 {
		return nil, 0, fmt.Errorf("stderr path empty")
	}

	return common.ReadFileAt(metadata.StderrPath, offset)
}

func getMicroVMMetadata(d common.Deployment) (*MicroVMEntryMetadata, error) {
	var metadata MicroVMEntryMetadata
	if tmp, err := json.Marshal(d.Metadata); err != nil {
		slog.Error("ProcessDriver.getMetadata", "error", "error on marshaling metadata", "deployment", d.ID)
		return nil, err
	} else {
		if err := json.Unmarshal(tmp, &metadata); err != nil {
			slog.Error("ProcessDriver.getMetadata", "error", "error on unmarshalling metadata", "deployment", d.ID)
			return nil, err
		}
	}
	return &metadata, nil
}

func (m *MicroVMDriver) buildBootArgs() string {
	args := fmt.Sprintf("%s %s", defaultBootArgs, strings.TrimSpace(m.config.BootArgs))
	args = strings.TrimSpace(args)

	if !strings.Contains(args, "root=") {
		if dev, ok := rootDevice(m.config.Drives); ok {
			args += " root=" + dev
			if !m.config.Drives[rootDriveIndex(m.config.Drives)].ReadOnly {
				args += " rw"
			}
		}
	}

	slog.Info("MicroVMDriver.buildBootArgs", "args", args)

	return args
}

func (m *MicroVMDriver) teardownVMM(socket string, pid int) error {
	slog.Info("MicroVMDriver.teardownVMM", "socket", socket, "pid", pid)

	if err := killVMM(pid); err != nil {
		return fmt.Errorf("VMM not killed pid=%d error=%s", pid, err.Error())
	}
	waitVMMExit(pid, killTimeout)

	return nil
}

func (m *MicroVMDriver) launch(metadata *MicroVMEntryMetadata) (int, error) {
	if metadata == nil {
		return 0, fmt.Errorf("nil metadata")
	}

	for i := range m.config.Netdev {
		if err := m.ensureTap(m.config.Netdev[i]); err != nil {
			return -1, err
		}
	}

	pid, err := m.spawnVMM(metadata)
	if err != nil {
		return -1, err
	}

	if err := m.configureVMM(metadata.APISocket, metadata.BootPlan); err != nil {
		return -1, m.teardownVMM(metadata.APISocket, pid)
	}

	if err := sendAction(metadata.APISocket, actionInstanceStart); err != nil {
		return -1, m.teardownVMM(metadata.APISocket, pid)
	}

	return pid, nil
}

func (m *MicroVMDriver) spawnVMM(metadata *MicroVMEntryMetadata) (int, error) {
	if metadata == nil {
		return 0, fmt.Errorf("nil metadata")
	}

	slog.Info("MicroVMDriver.spawnVMM", "msg", "spawning microvm", "metadata", *metadata)

	if err := os.Remove(metadata.APISocket); err != nil && !os.IsNotExist(err) {
		return 0, fmt.Errorf("MicroVMDriver: failed to remove stale API socket: %w", err)
	}

	stdoutFile, err := os.OpenFile(metadata.StdoutPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0640)
	if err != nil {
		return 0, fmt.Errorf("MicroVMDriver: failed to open stdout log file %s: %w", metadata.StdoutPath, err)
	}
	defer stdoutFile.Close()

	stderrFile, err := os.OpenFile(metadata.StderrPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0640)
	if err != nil {
		return 0, fmt.Errorf("MicroVMDriver: failed to open stderr log file %s: %w", metadata.StderrPath, err)
	}
	defer stderrFile.Close()

	cmd := exec.Command(m.config.Binary, "--api-sock", metadata.APISocket, "--id", metadata.VMName)
	cmd.Stdout = stdoutFile
	cmd.Stderr = stderrFile
	detachVMM(cmd)

	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("MicroVMDriver: failed to start %s: %w", m.config.Binary, err)
	}
	pid := cmd.Process.Pid

	// Reap the VMM when it exits so it does not linger as a zombie.
	go func() {
		if err := cmd.Wait(); err != nil {
			slog.Info("MicroVMDriver.spawnVMM", "msg", "VMM exited", "pid", pid, "error", err)
		}
	}()

	if err := waitAPIReady(metadata.APISocket, apiReadyTimeout); err != nil {
		_ = killVMM(pid)
		return 0, err
	}

	return pid, nil
}

func (m *MicroVMDriver) vcpus() int {
	if m.config.VCPUs > 0 {
		return m.config.VCPUs
	}
	return defaultVCPUs
}

func (m *MicroVMDriver) memoryMiB() int {
	if m.config.Memory > 0 {
		return m.config.Memory
	}
	return defaultMemoryMiB
}

func (m *MicroVMDriver) configureVMM(socket string, boot BootPlan) error {
	mc := fcMachineConfig{
		VCPUCount:  m.vcpus(),
		MemSizeMiB: m.memoryMiB(),
		SMT:        m.config.SMT,
	}
	if err := apiDo(socket, http.MethodPut, "/machine-config", mc, nil); err != nil {
		return err
	}

	bs := fcBootSource{
		KernelImagePath: boot.KernelPath,
		InitrdPath:      boot.InitrdPath,
		BootArgs:        boot.BootArgs,
	}
	if err := apiDo(socket, http.MethodPut, "/boot-source", bs, nil); err != nil {
		return err
	}

	// Drives are attached in configuration order so guest device names
	// (/dev/vda, /dev/vdb, ...) match the order the user wrote them in.
	for i := range m.config.Drives {
		overlay, ok := boot.Drives[i]
		if !ok {
			continue
		}
		id := driveID(m.config.Drives[i], i)
		d := fcDrive{
			DriveID:      id,
			PathOnHost:   overlay.FilePath,
			IsRootDevice: m.config.Drives[i].Root,
			IsReadOnly:   m.config.Drives[i].ReadOnly,
		}
		if err := apiDo(socket, http.MethodPut, "/drives/"+id, d, nil); err != nil {
			return err
		}
	}

	for i, nd := range m.config.Netdev {
		id := netdevID(nd, i)
		ni := fcNetworkInterface{
			IfaceID:     id,
			HostDevName: nd.Tap,
			GuestMAC:    nd.Mac,
		}
		if err := apiDo(socket, http.MethodPut, "/network-interfaces/"+id, ni, nil); err != nil {
			return err
		}
	}

	if b := m.config.Balloon; b != nil {
		balloon := fcBalloon{
			AmountMiB:             b.AmountMiB,
			DeflateOnOOM:          b.DeflateOnOOM,
			StatsPollingIntervalS: b.StatsPollingIntervalS,
		}
		// Without polling the statistics endpoint returns an error, which is
		// the only guest memory signal State() has.
		if balloon.StatsPollingIntervalS == 0 {
			balloon.StatsPollingIntervalS = 1
		}
		if err := apiDo(socket, http.MethodPut, "/balloon", balloon, nil); err != nil {
			return err
		}
	}

	return nil
}

func (m *MicroVMDriver) ensureTap(nd FCNetdev) error {
	if nd.Tap == "" {
		return fmt.Errorf("MicroVMDriver: netdev requires tap=<interface>")
	}

	link, err := netlink.LinkByName(nd.Tap)
	if err != nil {
		tap := &netlink.Tuntap{
			LinkAttrs: netlink.LinkAttrs{Name: nd.Tap},
			Mode:      netlink.TUNTAP_MODE_TAP,
			Flags:     netlink.TUNTAP_DEFAULTS,
		}
		if err := netlink.LinkAdd(tap); err != nil {
			return fmt.Errorf("MicroVMDriver: failed to create tap %s: %w", nd.Tap, err)
		}
		slog.Info("MicroVMDriver.ensureTap", "msg", "tap created", "tap", nd.Tap)
		if link, err = netlink.LinkByName(nd.Tap); err != nil {
			return fmt.Errorf("MicroVMDriver: tap %s not found after creation: %w", nd.Tap, err)
		}
	}

	if nd.BR != "" {
		bridge, err := netlink.LinkByName(nd.BR)
		if err != nil {
			return fmt.Errorf("MicroVMDriver: bridge %s not found: %w", nd.BR, err)
		}
		if link.Attrs().MasterIndex != bridge.Attrs().Index {
			if err := netlink.LinkSetMaster(link, bridge); err != nil {
				return fmt.Errorf("MicroVMDriver: failed to enslave tap %s to bridge %s: %w", nd.Tap, nd.BR, err)
			}
		}
	}

	if link.Attrs().Flags&net.FlagUp == 0 {
		if err := netlink.LinkSetUp(link); err != nil {
			return fmt.Errorf("MicroVMDriver: failed to bring tap %s up: %w", nd.Tap, err)
		}
	}

	return nil
}

func waitVMMExit(pid int, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !vmmAlive(pid) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}
func killVMM(pid int) error {
	if pid <= 0 {
		return nil
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Signal(syscall.SIGKILL)
}

func vmmAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

// detachVMM puts the VMM in its own session so it survives an agent restart
// and never receives the agent's terminal signals.
func detachVMM(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

func init() {
	_ = common.RegisterLoadDriver(&MicroVMDriver{})
}
