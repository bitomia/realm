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

const LinuxDriverID common.NodeDriverID = "linux"

var shutdownCmd = "/usr/sbin/shutdown"
var restartCmd = "/usr/sbin/shutdown"

type LinuxConfig struct {
	WakeOnLan bool   `json:"wol"`
	MAC       string `json:"MAC"`
}

type LinuxDriver struct {
	Config LinuxConfig
}

func NewLinuxDriverFromConfig(c *any) (common.NodeDriver, error) {
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
		Config: config,
	}, nil
}

func (l *LinuxDriver) DriverInfo() (common.NodeDriverInfo, error) {
	return common.NewNodeDriverInfo(
		LinuxDriverID,
		NewLinuxDriverFromConfig,
		false,
		common.WithStartMode(common.ClientMode),
	)
}

func (l *LinuxDriver) GetNodeDriverID() common.NodeDriverID {
	return LinuxDriverID
}

func (l *LinuxDriver) MarshalJSON() ([]byte, error) {
	return json.Marshal(l.GetDriverConfig())
}

func (l *LinuxDriver) UnmarshalJSON(data []byte) error {
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	var nodeDriver common.NodeDriver
	var err error
	if len(config) > 0 {
		var a any = config
		nodeDriver, err = NewLinuxDriverFromConfig(&a)
	} else {
		nodeDriver, err = NewLinuxDriverFromConfig(nil)
	}
	if err != nil {
		return err
	}
	*l = *nodeDriver.(*LinuxDriver)
	return nil
}

func (l *LinuxDriver) Provision(nodeName string, cloudInit *cloudinit.CloudInit, repository common.NodesRepository) error {
	// TODO
	// Verify commands as shutdown_cmd exists and other prerequisites

	if err := repository.SetSelf(nodeName, l, cloudInit, nil); err != nil {
		slog.Error("LinuxDriver.Provision", "msg", "failed to provision node", "error", err)
		return err
	}

	return nil
}

func (l *LinuxDriver) Deprovision(_ *string, repository common.NodesRepository) error {
	return repository.DeleteSelf()
}

func (l *LinuxDriver) GetDriverConfig() common.NodeDriverConfig {
	var c any = l.Config
	return common.NodeDriverConfig{Driver: LinuxDriverID, DriverConfig: &c}
}

func (l *LinuxDriver) Start(_ *string, repository common.NodesRepository) error {
	if !l.Config.WakeOnLan {
		return nil
	}

	return launchWakeOnLan(l.Config.MAC)
}

func (l *LinuxDriver) Stop(_ *string, message string, time uint32, repository common.NodesRepository, _ bool) error {
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

func (l *LinuxDriver) Restart(_ *string, message string, time uint32, repository common.NodesRepository) error {
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

func (l *LinuxDriver) UpdateStatus(_ *string, repository common.NodesRepository) (common.NodeStatus, error) {
	return common.NodeStatus{StatusCode: common.NodeStatusReady, Reason: ""}, nil
}

func (l *LinuxDriver) GetState(_ *string, _ common.NodesRepository) (common.NodeState, error) {
	return cpu.GetNodeState()
}
