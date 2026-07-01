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

const LinuxDriverID common.NodeDriverID = "linux"

var shutdownCmd = "/usr/sbin/shutdown"
var restartCmd = "/usr/sbin/shutdown"

type LinuxConfig struct {
	WakeOnLan bool   `json:"wol"`
	MAC       string `json:"MAC"`
}

type LinuxDriver struct {
	config LinuxConfig
	ctx    common.NodeContext
}

func NewLinuxDriverFromConfig(ctx common.NodeContext, c *any) (common.NodeDriver, error) {
	// Set default values
	var config = LinuxConfig{
		WakeOnLan: false,
		MAC:       "",
	}
	if c != nil {
		// Configure mapstructure decoder to use 'json' tags
		// because it has to work for config files (yaml)
		// and for remote commands (json)
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

	return &LinuxDriver{
		config: config,
	}, nil
}

func (l *LinuxDriver) Info() (common.NodeDriverInfo, error) {
	return common.NewNodeDriverInfo(
		LinuxDriverID,
		NewLinuxDriverFromConfig,
		common.WithPowerOnMode(common.ClientMode),
	)
}

func (l *LinuxDriver) ID() common.NodeDriverID {
	return LinuxDriverID
}

func (l *LinuxDriver) Register(nodeName string, cloudInit *cloudinit.CloudInit) error {
	// TODO
	// Verify commands as shutdown_cmd exists and other prerequisites

	if err := l.ctx.Repository.SetSelf(nodeName, l, cloudInit, nil); err != nil {
		slog.Error("LinuxDriver.Register", "msg", "failed to register node", "error", err)
		return err
	}

	return nil
}

func (l *LinuxDriver) Unregister(_ *string) error {
	return l.ctx.Repository.DeleteSelf()
}

func (l *LinuxDriver) Config() common.NodeDriverConfig {
	var c any = l.config
	return common.NodeDriverConfig{Driver: LinuxDriverID, DriverConfig: &c}
}

func (l *LinuxDriver) PowerOn(_ *string) error {
	if !l.config.WakeOnLan {
		return nil
	}

	return launchWakeOnLan(l.config.MAC)
}

func (l *LinuxDriver) PowerOff(_ *string) error {
	cmd := exec.Command(shutdownCmd, "-h", "now")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to execute shutdown command: %w", err)
	}
	return nil
}

func (l *LinuxDriver) Shutdown(_ *string, message string, time uint32) error {
	timeArg := "now"
	if time > 0 {
		timeArg = fmt.Sprintf("+%d", time)
	}

	cmd := exec.Command(shutdownCmd, "-P", timeArg, message)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to execute shutdown command: %w", err)
	}
	return nil
}

func (l *LinuxDriver) Restart(_ *string, message string, time uint32) error {
	timeArg := "now"
	if time > 0 {
		timeArg = fmt.Sprintf("+%d", time)
	}

	cmd := exec.Command(restartCmd, "-r", timeArg, message)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to execute restart command: %w", err)
	}
	return nil
}

func (l *LinuxDriver) RefreshStatus(_ *string) (common.NodeStatus, error) {
	return common.NodeStatus{StatusCode: common.NodeStatusReady, Reason: ""}, nil
}

func (l *LinuxDriver) State(_ *string) (common.NodeState, error) {
	return cpu.GetNodeState()
}
