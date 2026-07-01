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
	config WindowsConfig
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
		config: config,
	}, nil
}

func (w *WindowsDriver) Info() (common.NodeDriverInfo, error) {
	return common.NewNodeDriverInfo(
		WindowsDriverID,
		NewWindowsDriverFromConfig,
		common.WithPowerOnMode(common.ClientMode),
	)
}

func (w *WindowsDriver) ID() common.NodeDriverID {
	return WindowsDriverID
}

func (w *WindowsDriver) Register(nodeName string, cloudInit *cloudinit.CloudInit) error {
	if err := w.ctx.Repository.SetSelf(nodeName, w, cloudInit, nil); err != nil {
		slog.Error("WindowsDriver.Register", "msg", "failed to register node", "error", err)
		return err
	}

	return nil
}

func (w *WindowsDriver) Unregister(_ *string) error {
	return w.ctx.Repository.DeleteSelf()
}

func (w *WindowsDriver) Config() common.NodeDriverConfig {
	var c any = w.config
	return common.NodeDriverConfig{Driver: WindowsDriverID, DriverConfig: &c}
}

func (w *WindowsDriver) PowerOn(_ *string) error {
	if !w.config.WakeOnLan {
		return nil
	}

	return launchWakeOnLan(w.config.MAC)
}

func (w *WindowsDriver) PowerOff(_ *string) error {
	cmd := exec.Command(windowsShutdownCmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to execute shutdown command: %w", err)
	}
	return nil
}

func (w *WindowsDriver) Shutdown(_ *string, message string, time uint32) error {
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

func (w *WindowsDriver) RefreshStatus(_ *string) (common.NodeStatus, error) {
	return common.NodeStatus{StatusCode: common.NodeStatusReady, Reason: ""}, nil
}

func (l *WindowsDriver) State(_ *string) (common.NodeState, error) {
	return cpu.GetNodeState()
}
