package nodes

import (
	"fmt"
	"log/slog"
	"net"
	"os/exec"

	"github.com/go-viper/mapstructure/v2"

	"github.com/bitomia/realm/agent/cpu"
	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/common/cloudinit"
)

const WindowsDriverID common.NodeDriverID = "windows"

var windowsShutdownCmd = "shutdown.exe"

type WindowsConfig struct {
	WakeOnLan bool   `json:"wol"`
	MAC       string `json:"MAC"`
}

type WindowsDriver struct {
	Config WindowsConfig
	ctx    common.NodeContext
}

func NewWindowsDriverFromConfig(ctx common.NodeContext, c *any) (common.NodeDriver, error) {
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
		common.WithStartMode(common.ClientMode),
	)
}

func (w *WindowsDriver) GetNodeDriverID() common.NodeDriverID {
	return WindowsDriverID
}

func (w *WindowsDriver) Provision(nodeName string, cloudInit *cloudinit.CloudInit) error {
	if err := w.ctx.Repository.SetSelf(nodeName, w, cloudInit, nil); err != nil {
		slog.Error("WindowsDriver.Provision", "msg", "failed to provision node", "error", err)
		return err
	}

	return nil
}

func (w *WindowsDriver) Deprovision(_ *string) error {
	return w.ctx.Repository.DeleteSelf()
}

func (w *WindowsDriver) GetDriverConfig() common.NodeDriverConfig {
	var c any = w.Config
	return common.NodeDriverConfig{Driver: WindowsDriverID, DriverConfig: &c}
}

func (w *WindowsDriver) Start(_ *string) error {
	if !w.Config.WakeOnLan {
		return nil
	}

	return launchWakeOnLan(w.Config.MAC)
}

func (w *WindowsDriver) Stop(_ *string, message string, time uint32, _ bool) error {
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

func (w *WindowsDriver) Restart(_ *string, message string, time uint32) error {
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

func (w *WindowsDriver) UpdateStatus(_ *string) (common.NodeStatus, error) {
	return common.NodeStatus{StatusCode: common.NodeStatusReady, Reason: ""}, nil
}

func (l *WindowsDriver) GetState(_ *string) (common.NodeState, error) {
	return cpu.GetNodeState()
}
