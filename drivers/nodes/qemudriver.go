package nodes

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/go-viper/mapstructure/v2"

	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/daemon/capabilities"
	"github.com/bitomia/realm/daemon/config"
)

const QemuDriverID common.NodeDriverID = "qemu"

type QemuDrive struct {
	File   string `json:"file,omitempty"`
	Format string `json:"format,omitempty"`
	If     string `json:"if,omitempty"`
	Media  string `json:"media,omitempty"`
	Index  string `json:"index,omitempty"`
}

type QemuNetdev struct {
	Type       string `json:"type,omitempty"`
	ID         string `json:"id,omitempty"`
	Ifname     string `json:"ifname,omitempty"`
	Script     string `json:"script,omitempty"`
	Downscript string `json:"downscript,omitempty"`
	BR         string `json:"br,omitempty"`
	Helper     string `json:"helper,omitempty"`
	Net        string `json:"net,omitempty"`
	DHCPStart  string `json:"dhcpstart,omitempty"`
	Hostfwd    string `json:"hostfwd,omitempty"`
}

type QemuConfig struct {
	Emulator string       `json:"emulator"`
	Machine  string       `json:"machine,omitempty"`
	Accel    string       `json:"accel,omitempty"`
	CPU      string       `json:"cpu,omitempty"`
	Memory   int          `json:"memory,omitempty"`
	SMP      string       `json:"smp,omitempty"`
	Serial   string       `json:"serial,omitempty"`
	Params   []string     `json:"params,omitempty"`
	Drives   []QemuDrive  `json:"drives,omitempty"`
	Netdevs  []QemuNetdev `json:"netdevs,omitempty"`
	QMPPort  int          `json:"qmp_port,omitempty"`
}

type QemuNodeMetadata struct {
	Pid     int `json:"pid,omitempty"`
	QMPPort int `json:"qmp_port,omitempty"`
}

type QemuDriver struct {
	Config QemuConfig
}

func NewQemuDriverFromConfig(c *any) (common.NodeDriver, error) {
	var cfg QemuConfig
	if c != nil {
		decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
			TagName: "json",
			Result:  &cfg,
		})
		if err != nil {
			return nil, err
		}
		if err := decoder.Decode(*c); err != nil {
			return nil, err
		}
	}

	if cfg.Emulator == "" {
		return nil, fmt.Errorf("qemu: emulator path is required")
	}

	return &QemuDriver{
		Config: cfg,
	}, nil
}

func (q *QemuDriver) DriverInfo() (common.NodeDriverInfo, error) {
	return common.NewNodeDriverInfo(
		QemuDriverID,
		NewQemuDriverFromConfig,
		true,
		common.WithStartMode(common.DaemonMode),
	)
}

func (q *QemuDriver) GetNodeDriverID() common.NodeDriverID {
	return QemuDriverID
}

func (q *QemuDriver) Provision(nodeName string, repository common.NodesRepository) error {
	resolved, err := common.ResolveExecPath(q.Config.Emulator, nil)
	if err != nil {
		return err
	}
	q.Config.Emulator = resolved

	dataPath := config.Get().Daemon.DataPath

	qmpPort, err := getFreePort()
	if err != nil {
		return fmt.Errorf("qemu: failed to find free port for QMP: %w", err)
	}

	args := q.buildArgs(nodeName, qmpPort)
	args = append(args, "-S")
	cmd := exec.Command(q.Config.Emulator, args...)

	slog.Info("QemuDriver.Provision", "msg", "launching qemu", "node", nodeName, "cmd", q.Config.Emulator, "args", args)

	stdoutPath := filepath.Join(dataPath, "logs", "qemu", nodeName+"_stdout.log")
	stderrPath := filepath.Join(dataPath, "logs", "qemu", nodeName+"_stderr.log")

	stdoutFile, err := common.CreateLogFile(stdoutPath, 0755)
	if err != nil {
		return fmt.Errorf("qemu: failed to create stdout log: %w", err)
	}
	stderrFile, err := common.CreateLogFile(stderrPath, 0755)
	if err != nil {
		stdoutFile.Close()
		return fmt.Errorf("qemu: failed to create stderr log: %w", err)
	}

	cmd.Stdout = stdoutFile
	cmd.Stderr = stderrFile

	if err := cmd.Start(); err != nil {
		stdoutFile.Close()
		stderrFile.Close()
		return fmt.Errorf("qemu: failed to start emulator: %w", err)
	}

	if err := waitForQMP(qmpPort, cmd); err != nil {
		cmd.Process.Kill()
		cmd.Wait()
		stdoutFile.Close()
		stderrFile.Close()
		stderrContent, readErr := os.ReadFile(stderrPath)
		if readErr == nil && len(stderrContent) > 0 {
			return fmt.Errorf("qemu: VM failed to start: %w\nstderr: %s", err, string(stderrContent))
		}
		return fmt.Errorf("qemu: VM failed to start: %w", err)
	}

	q.Config.QMPPort = qmpPort

	metadata := QemuNodeMetadata{
		Pid:     cmd.Process.Pid,
		QMPPort: qmpPort,
	}

	slog.Info("QemuDriver.Provision", "msg", "qemu started in paused state", "pid", metadata.Pid, "qmp_port", qmpPort)

	if err := repository.SetGuestNode(nodeName, q, metadata); err != nil {
		cmd.Process.Kill()
		cmd.Wait()
		stdoutFile.Close()
		stderrFile.Close()
		slog.Error("QemuDriver.Provision", "msg", "failed to provision node", "error", err)
		return err
	}

	return nil
}

func (q *QemuDriver) Deprovision(nodeName *string, repository common.NodesRepository) error {
	if nodeName == nil {
		return fmt.Errorf("QemuDriver expects node name for guest node deprovision")
	}

	if q.Config.QMPPort != 0 {
		if err := qmpQuit(q.Config.QMPPort); err != nil {
			slog.Warn("QemuDriver.Deprovision", "msg", "failed to quit via QMP, will attempt process kill", "error", err)
		}
	}

	self, err := repository.GetGuestNode(*nodeName)
	if err == nil {
		if metadata, ok := self.Metadata.(*QemuNodeMetadata); ok && metadata.Pid != 0 {
			if proc, err := os.FindProcess(metadata.Pid); err == nil {
				proc.Kill()
			}
		} else if metadataMap, ok := self.Metadata.(map[string]any); ok {
			if pid, ok := metadataMap["pid"].(float64); ok && pid != 0 {
				if proc, err := os.FindProcess(int(pid)); err == nil {
					proc.Kill()
				}
			}
		}
	}

	return repository.DeleteGuestNode(*nodeName, q, self.Metadata)
}

func (q *QemuDriver) GetDriverConfig() common.NodeDriverConfig {
	var c any = q.Config
	return common.NodeDriverConfig{Driver: QemuDriverID, DriverConfig: &c}
}

func (q *QemuDriver) MarshalJSON() ([]byte, error) {
	return json.Marshal(q.GetDriverConfig())
}

func (q *QemuDriver) UnmarshalJSON(data []byte) error {
	var cfgMap map[string]any
	if err := json.Unmarshal(data, &cfgMap); err != nil {
		return err
	}

	var nodeDriver common.NodeDriver
	var err error
	if len(cfgMap) > 0 {
		var a any = cfgMap
		nodeDriver, err = NewQemuDriverFromConfig(&a)
	} else {
		nodeDriver, err = NewQemuDriverFromConfig(nil)
	}
	if err != nil {
		return err
	}
	*q = *nodeDriver.(*QemuDriver)
	return nil
}

func (q *QemuDriver) buildArgs(nodeName string, qmpPort int) []string {
	var args []string

	args = append(args, "-name", nodeName)
	args = append(args, "-qmp", fmt.Sprintf("tcp:127.0.0.1:%d,server,nowait", qmpPort))

	if q.Config.Machine != "" {
		args = append(args, "-machine", q.Config.Machine)
	}
	if q.Config.Accel != "" {
		args = append(args, "-accel", q.Config.Accel)
	}
	if q.Config.CPU != "" {
		args = append(args, "-cpu", q.Config.CPU)
	}
	if q.Config.Memory > 0 {
		args = append(args, "-m", fmt.Sprintf("%d", q.Config.Memory))
	}
	if q.Config.SMP != "" {
		args = append(args, "-smp", q.Config.SMP)
	}
	if q.Config.Serial != "" {
		args = append(args, "-serial", q.Config.Serial)
	}

	for _, drive := range q.Config.Drives {
		driveStr := ""
		if drive.File != "" {
			driveStr += "file=" + drive.File
		}
		if drive.Format != "" {
			if driveStr != "" {
				driveStr += ","
			}
			driveStr += "format=" + drive.Format
		}
		if drive.If != "" {
			if driveStr != "" {
				driveStr += ","
			}
			driveStr += "if=" + drive.If
		}
		if drive.Media != "" {
			if driveStr != "" {
				driveStr += ","
			}
			driveStr += "media=" + drive.Media
		}
		if drive.Index != "" {
			if driveStr != "" {
				driveStr += ","
			}
			driveStr += "index=" + drive.Index
		}
		if driveStr != "" {
			args = append(args, "-drive", driveStr)
		}
	}

	for _, nd := range q.Config.Netdevs {
		ndStr := ""
		if nd.Type != "" {
			ndStr += nd.Type
		}
		if nd.ID != "" {
			if ndStr != "" {
				ndStr += ","
			}
			ndStr += "id=" + nd.ID
		}
		if nd.Ifname != "" {
			if ndStr != "" {
				ndStr += ","
			}
			ndStr += "ifname=" + nd.Ifname
		}
		if nd.Script != "" {
			if ndStr != "" {
				ndStr += ","
			}
			ndStr += "script=" + nd.Script
		}
		if nd.Downscript != "" {
			if ndStr != "" {
				ndStr += ","
			}
			ndStr += "downscript=" + nd.Downscript
		}
		if nd.BR != "" {
			if ndStr != "" {
				ndStr += ","
			}
			ndStr += "br=" + nd.BR
		}
		if nd.Helper != "" {
			if ndStr != "" {
				ndStr += ","
			}
			ndStr += "helper=" + nd.Helper
		}
		if nd.Net != "" {
			if ndStr != "" {
				ndStr += ","
			}
			ndStr += "net=" + nd.Net
		}
		if nd.DHCPStart != "" {
			if ndStr != "" {
				ndStr += ","
			}
			ndStr += "dhcpstart=" + nd.DHCPStart
		}
		if nd.Hostfwd != "" {
			if ndStr != "" {
				ndStr += ","
			}
			ndStr += "hostfwd=" + nd.Hostfwd
		}
		if ndStr != "" {
			args = append(args, "-netdev", ndStr)
		}
	}

	args = append(args, q.Config.Params...)

	return args
}

func (q *QemuDriver) Start(nodeName *string, repository common.NodesRepository) error {
	if nodeName == nil {
		return fmt.Errorf("nodeName cannot be nil")
	}

	metadata, err := q.getMetadata(*nodeName, repository)
	if err != nil {
		return fmt.Errorf("qemu: failed to get metadata: %w", err)
	}

	if metadata.QMPPort == 0 {
		return fmt.Errorf("qemu: no QMP port available, VM may not be provisioned")
	}

	slog.Info("QemuDriver.Start", "msg", "resuming paused VM", "node", *nodeName, "qmp_port", metadata.QMPPort)

	if err := qmpCont(metadata.QMPPort); err != nil {
		return fmt.Errorf("qemu: failed to resume VM: %w", err)
	}

	slog.Info("QemuDriver.Start", "msg", "VM resumed", "node", *nodeName)

	return nil
}

func (q *QemuDriver) Stop(nodeName *string, _ string, _ uint32, repository common.NodesRepository, force bool) error {
	if nodeName == nil {
		return fmt.Errorf("nodeName cannot be nil")
	}
	metadata, err := q.getMetadata(*nodeName, repository)
	if err != nil {
		return fmt.Errorf("qemu: failed to get metadata: %w", err)
	}
	if metadata.QMPPort == 0 {
		return fmt.Errorf("qemu: no QMP port available")
	}
	if force {
		if err := qmpQuit(metadata.QMPPort); err != nil {
			return err
		}
	} else {
		if err := qmpStop(metadata.QMPPort); err != nil {
			return err
		}
	}
	return nil
}

func (q *QemuDriver) Restart(nodeName *string, message string, time uint32, repository common.NodesRepository) error {
	if nodeName == nil {
		return fmt.Errorf("nodeName cannot be nil")
	}

	metadata, err := q.getMetadata(*nodeName, repository)
	if err != nil {
		return fmt.Errorf("getMetadata on Shutdown failed: %s", err.Error())
	}

	if metadata.QMPPort == 0 {
		return fmt.Errorf("qemu: no QMP port available")
	}

	return qmpSystemReset(metadata.QMPPort)
}

func (q *QemuDriver) UpdateStatus(nodeName *string, repository common.NodesRepository) (common.NodeStatus, error) {
	if nodeName == nil {
		error := fmt.Errorf("getMetadata on UpdateStatus failed: nodeName cannot be nil")
		return common.NodeStatus{StatusCode: common.NodeStatusError, Reason: error.Error()}, error
	}

	metadata, err := q.getMetadata(*nodeName, repository)
	if err != nil {
		error := fmt.Errorf("getMetadata on UpdateStatus failed: %s", err.Error())
		return common.NodeStatus{StatusCode: common.NodeStatusError, Reason: error.Error()}, error
	}

	if metadata.QMPPort == 0 {
		return common.NodeStatus{StatusCode: common.NodeStatusReady, Reason: "not started"}, nil
	}

	running, err := qmpQueryStatus(metadata.QMPPort)
	if err != nil {
		return common.NodeStatus{StatusCode: common.NodeStatusError, Reason: err.Error()}, err
	}
	if running {
		return common.NodeStatus{StatusCode: common.NodeStatusReady, Reason: ""}, nil
	}

	return common.NodeStatus{StatusCode: common.NodeStatusOffline, Reason: "VM is not running"}, nil
}

func (q *QemuDriver) GetState() (common.NodeState, error) {
	state := common.NodeState{}

	if q.Config.QMPPort == 0 {
		return state, fmt.Errorf("qemu: no QMP port available, VM may not be running")
	}

	// Query vCPU count
	numCPU, err := qmpQueryCpusFast(q.Config.QMPPort)
	if err != nil {
		slog.Warn("QemuDriver.GetState", "msg", "failed to query CPUs via QMP", "error", err)
	} else {
		state.NumCPU = numCPU
	}

	// Query total memory via memory-size-summary
	totalMem, err := qmpQueryMemorySizeSummary(q.Config.QMPPort)
	if err != nil {
		slog.Warn("QemuDriver.GetState", "msg", "failed to query memory size via QMP", "error", err)
		// Fallback to configured memory (MB to bytes)
		if q.Config.Memory > 0 {
			state.TotalMem = uint64(q.Config.Memory) * 1024 * 1024
		}
	} else {
		state.TotalMem = totalMem
	}

	// Query balloon for actual memory usage (requires virtio-balloon device)
	balloonMem, err := qmpQueryBalloon(q.Config.QMPPort)
	if err != nil {
		slog.Debug("QemuDriver.GetState", "msg", "balloon query unavailable", "error", err)
	} else if state.TotalMem > 0 {
		state.UsedMem = balloonMem
		state.FreeMem = state.TotalMem - balloonMem
		state.FreeMemPercent = float64(state.FreeMem) / float64(state.TotalMem) * 100
	}

	return state, nil
}

func (q *QemuDriver) GetCapabilities() (common.Capabilities, error) {
	daemonCaps := capabilities.Get()
	if daemonCaps == nil {
		return nil, fmt.Errorf("Daemon capabilities not initialized")
	}
	return daemonCaps, nil
}

func (q *QemuDriver) getMetadata(nodeName string, repository common.NodesRepository) (*QemuNodeMetadata, error) {
	node, err := repository.GetGuestNode(nodeName)
	if err != nil {
		return nil, err
	}

	var metadata QemuNodeMetadata
	if tmp, err := json.Marshal(node.Metadata); err != nil {
		slog.Error("QemuDriver.getMetadata", "error", "error on marshalling metadata", "node", nodeName)
		return nil, err
	} else {
		if err := json.Unmarshal(tmp, &metadata); err != nil {
			slog.Error("QemuDriver.getMetadata", "error", "error on unmarshalling metadata", "node", nodeName)
			return nil, err
		}
	}

	return &metadata, nil
}
