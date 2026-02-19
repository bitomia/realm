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
	"github.com/bitomia/realm/common/config"
	"github.com/bitomia/realm/daemon/capabilities"
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
	CPU      string       `json:"cpu,omitempty"`
	Memory   int          `json:"memory,omitempty"`
	SMP      string       `json:"smp,omitempty"`
	Serial   string       `json:"serial,omitempty"`
	Params   []string     `json:"params,omitempty"`
	Drives   []QemuDrive  `json:"drives,omitempty"`
	Netdevs  []QemuNetdev `json:"netdevs,omitempty"`
}

type QemuNodeMetadata struct {
	Pid       int    `json:"pid,omitempty"`
	QMPSocket string `json:"qmp_socket,omitempty"`
}

type QemuDriver struct {
	Config   QemuConfig
	nodeName string
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
		common.WithStartupMode(common.DaemonMode),
	)
}

func (q *QemuDriver) GetNodeDriverID() common.NodeDriverID {
	return QemuDriverID
}

func (q *QemuDriver) Provision(nodeName string, repository common.NodesRepository) error {
	if _, err := exec.LookPath(q.Config.Emulator); err != nil {
		return fmt.Errorf("qemu: emulator binary not found: %w", err)
	}

	q.nodeName = nodeName
	dataPath := config.Get().Daemon.DataPath
	qmpSocketPath := filepath.Join(dataPath, "qemu", nodeName+".qmp")

	// Ensure QMP socket directory exists
	if err := os.MkdirAll(filepath.Dir(qmpSocketPath), 0755); err != nil {
		return fmt.Errorf("qemu: failed to create qmp socket directory: %w", err)
	}

	// Build QEMU command
	args := q.buildArgs()
	cmd := exec.Command(q.Config.Emulator, args...)

	// Set up stdout/stderr log files
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

	slog.Info("QemuDriver.Provision", "emulator", q.Config.Emulator, "args", args)

	if err := cmd.Start(); err != nil {
		stdoutFile.Close()
		stderrFile.Close()
		return fmt.Errorf("qemu: failed to start emulator: %w", err)
	}

	meta := QemuNodeMetadata{
		Pid:       cmd.Process.Pid,
		QMPSocket: qmpSocketPath,
	}

	slog.Info("QemuDriver.Provision", "pid", meta.Pid, "qmp_socket", qmpSocketPath)

	if err := repository.SetSelf(nodeName, q, meta); err != nil {
		slog.Error("QemuDriver.Provision", "msg", "failed to provision node", "error", err)
		return err
	}

	return nil
}

func (q *QemuDriver) Deprovision(repository common.NodesRepository) error {
	return repository.DeleteSelf()
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

func getMetadataFromRepo(repository common.NodesRepository) (QemuNodeMetadata, error) {
	entry, err := repository.GetSelf()
	if err != nil {
		return QemuNodeMetadata{}, fmt.Errorf("qemu: failed to get node entry: %w", err)
	}

	var meta QemuNodeMetadata
	metaBytes, err := json.Marshal(entry.Metadata)
	if err != nil {
		return QemuNodeMetadata{}, fmt.Errorf("qemu: failed to marshal metadata: %w", err)
	}
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		return QemuNodeMetadata{}, fmt.Errorf("qemu: failed to unmarshal metadata: %w", err)
	}

	return meta, nil
}

func (q *QemuDriver) buildArgs() []string {
	dataPath := config.Get().Daemon.DataPath
	qmpSocketPath := filepath.Join(dataPath, "qemu", q.nodeName+".qmp")

	var args []string

	args = append(args, "-name", q.nodeName)
	args = append(args, "-qmp", fmt.Sprintf("unix:%s,server,nowait", qmpSocketPath))

	if q.Config.Machine != "" {
		args = append(args, "-machine", q.Config.Machine)
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

func (q *QemuDriver) Startup(repository common.NodesRepository) error {
	fmt.Println("HERE")
	return nil
}

func (q *QemuDriver) Shutdown(message string, time uint32, repository common.NodesRepository) error {
	meta, err := getMetadataFromRepo(repository)
	if err != nil {
		return err
	}
	if meta.QMPSocket == "" {
		return fmt.Errorf("qemu: no QMP socket path available")
	}
	return qmpSystemPowerdown(meta.QMPSocket)
}

func (q *QemuDriver) Restart(message string, time uint32, repository common.NodesRepository) error {
	meta, err := getMetadataFromRepo(repository)
	if err != nil {
		return err
	}
	if meta.QMPSocket == "" {
		return fmt.Errorf("qemu: no QMP socket path available")
	}
	return qmpSystemReset(meta.QMPSocket)
}

func (q *QemuDriver) UpdateStatus(repository common.NodesRepository) (common.NodeStatus, error) {
	meta, err := getMetadataFromRepo(repository)
	if err != nil {
		return common.NodeStatus{StatusCode: common.NodeStatusError, Reason: err.Error()}, nil
	}
	if meta.QMPSocket == "" {
		return common.NodeStatus{StatusCode: common.NodeStatusReady, Reason: "not started"}, nil
	}

	running, err := qmpQueryStatus(meta.QMPSocket)
	if err != nil {
		return common.NodeStatus{StatusCode: common.NodeStatusError, Reason: err.Error()}, nil
	}
	if running {
		return common.NodeStatus{StatusCode: common.NodeStatusReady, Reason: ""}, nil
	}
	return common.NodeStatus{StatusCode: common.NodeStatusError, Reason: "VM is not running"}, nil
}

func (q *QemuDriver) GetCapabilities() (common.Capabilities, error) {
	daemonCaps := capabilities.Get()
	if daemonCaps == nil {
		return nil, fmt.Errorf("Daemon capabilities not initialized")
	}
	return daemonCaps, nil
}
