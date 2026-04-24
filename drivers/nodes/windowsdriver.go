package nodes

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os/exec"

	"github.com/go-viper/mapstructure/v2"

	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/common/cloudinit"
	"github.com/bitomia/realm/daemon/cpu"
)

const WindowsDriverID common.NodeDriverID = "windows"

var windowsShutdownCmd = "shutdown.exe"

type WindowsConfig struct {
	WakeOnLan bool   `json:"wol"`
	MAC       string `json:"MAC"`
}

type WindowsDriver struct {
	Config WindowsConfig
}

func NewWindowsDriverFromConfig(c *any) (common.NodeDriver, error) {
	var config = WindowsConfig{
		WakeOnLan: false,
		MAC:       "",
	}
	if c != nil {
		decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
			TagName: "json",
			Result:  &config,
		})
		if err != nil {
			return nil, err
		}
		if err := decoder.Decode(*c); err != nil {
			return nil, err
		}
	}

	if config.WakeOnLan {
		if config.MAC == "" {
			return nil, fmt.Errorf("mac address required when wol is enabled")
		}
		if _, err := net.ParseMAC(config.MAC); err != nil {
			return nil, fmt.Errorf("invalid mac address: %w", err)
		}
	}

	return &WindowsDriver{
		Config: config,
	}, nil
}

func (w *WindowsDriver) DriverInfo() (common.NodeDriverInfo, error) {
	return common.NewNodeDriverInfo(
		WindowsDriverID,
		NewWindowsDriverFromConfig,
		false,
		common.WithStartMode(common.ClientMode),
	)
}

func (w *WindowsDriver) GetNodeDriverID() common.NodeDriverID {
	return WindowsDriverID
}

func (w *WindowsDriver) MarshalJSON() ([]byte, error) {
	return json.Marshal(w.GetDriverConfig())
}

func (w *WindowsDriver) UnmarshalJSON(data []byte) error {
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	var nodeDriver common.NodeDriver
	var err error
	if len(config) > 0 {
		var a any = config
		nodeDriver, err = NewWindowsDriverFromConfig(&a)
	} else {
		nodeDriver, err = NewWindowsDriverFromConfig(nil)
	}
	if err != nil {
		return err
	}
	*w = *nodeDriver.(*WindowsDriver)
	return nil
}

func (w *WindowsDriver) Provision(nodeName string, cloudInit *cloudinit.CloudInit, repository common.NodesRepository) error {
	if err := repository.SetSelf(nodeName, w, cloudInit, nil); err != nil {
		slog.Error("WindowsDriver.Provision", "msg", "failed to provision node", "error", err)
		return err
	}

	return nil
}

func (w *WindowsDriver) Deprovision(_ *string, repository common.NodesRepository) error {
	return repository.DeleteSelf()
}

func (w *WindowsDriver) GetDriverConfig() common.NodeDriverConfig {
	var c any = w.Config
	return common.NodeDriverConfig{Driver: WindowsDriverID, DriverConfig: &c}
}

func (w *WindowsDriver) Start(_ *string, repository common.NodesRepository) error {
	if !w.Config.WakeOnLan {
		return nil
	}

	return launchWakeOnLan(w.Config.MAC)
}

func (w *WindowsDriver) Stop(_ *string, message string, time uint32, repository common.NodesRepository, _ bool) error {
	args := []string{"/s", "/t", fmt.Sprintf("%d", time)}
	if message != "" {
		args = append(args, "/c", message)
	}

	cmd := exec.Command(windowsShutdownCmd, args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to execute shutdown command: %w", err)
	}
	return nil
}

func (w *WindowsDriver) Restart(_ *string, message string, time uint32, repository common.NodesRepository) error {
	args := []string{"/r", "/t", fmt.Sprintf("%d", time)}
	if message != "" {
		args = append(args, "/c", message)
	}

	cmd := exec.Command(windowsShutdownCmd, args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to execute restart command: %w", err)
	}
	return nil
}

func (w *WindowsDriver) UpdateStatus(_ *string, repository common.NodesRepository) (common.NodeStatus, error) {
	return common.NodeStatus{StatusCode: common.NodeStatusReady, Reason: ""}, nil
}

func (l *WindowsDriver) GetState(_ *string, _ common.NodesRepository) (common.NodeState, error) {
	return cpu.GetNodeState()
}
