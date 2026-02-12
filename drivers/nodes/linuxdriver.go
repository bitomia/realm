package nodes

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os/exec"

	"github.com/go-viper/mapstructure/v2"

	"github.com/bitomia/realm/common"
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

	driver := &LinuxDriver{
		Config: config,
	}
	if err := driver.Verify(); err != nil {
		return nil, err
	}
	return driver, nil
}

func (l LinuxDriver) DriverInfo() common.NodeDriverInfo {
	return common.NodeDriverInfo{
		ID:  LinuxDriverID,
		New: NewLinuxDriverFromConfig,
	}
}

func (l LinuxDriver) GetNodeDriverID() common.NodeDriverID {
	return LinuxDriverID
}

func (l LinuxDriver) MarshalJSON() ([]byte, error) {
	return json.Marshal(l.GetDriverConfig())
}

func (l LinuxDriver) UnmarshalJSON(data []byte) error {
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
	} else {
		l = nodeDriver.(LinuxDriver)
		return nil
	}
}

func (l LinuxDriver) Verify() error {
	if l.Config.WakeOnLan {
		if l.Config.MAC == "" {
			return fmt.Errorf("mac address required when wol is enabled")
		}
		if _, err := net.ParseMAC(l.Config.MAC); err != nil {
			return fmt.Errorf("invalid mac address: %w", err)
		}
	}
	return nil
}

func (l LinuxDriver) Plan(nodeName string, repository common.NodesRepository) error {
	// TODO
	// Verify commands as shutdown_cmd exists and other prerequisites

	// TODO
	// Verify nodeName is not already registered and warn about replacing

	if err := repository.Create(nodeName, l, nil); err != nil {
		slog.Error("LinuxDriver.Plan", "msg", "failed to plan node", "error", err)
		return err
	}

	return nil
}

func (l LinuxDriver) GetDriverConfig() common.NodeDriverConfig {
	var c any = l.Config
	return common.NodeDriverConfig{Driver: LinuxDriverID, DriverConfig: &c}
}

func (l LinuxDriver) Startup() error {
	if !l.Config.WakeOnLan {
		return nil
	}

	mac, err := net.ParseMAC(l.Config.MAC)
	if err != nil {
		return fmt.Errorf("invalid mac address: %w", err)
	}

	// Build the magic packet: 6 bytes of 0xFF followed by 16 repetitions of the MAC address
	packet := make([]byte, 102)
	for i := 0; i < 6; i++ {
		packet[i] = 0xFF
	}
	for i := 0; i < 16; i++ {
		copy(packet[6+i*6:], mac)
	}

	// Send the magic packet as UDP broadcast on port 9
	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{
		IP:   net.IPv4bcast,
		Port: 9,
	})
	if err != nil {
		return fmt.Errorf("failed to create udp connection: %w", err)
	}
	defer conn.Close()

	if _, err := conn.Write(packet); err != nil {
		return fmt.Errorf("failed to send wol packet: %w", err)
	}

	return nil
}

func (l LinuxDriver) Shutdown(message string, time uint32) error {
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

func (l LinuxDriver) Restart(message string, time uint32) error {
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

func (l LinuxDriver) GetStatus() (common.NodeStatus, error) {
	// If this method is being called, the daemon is running on this node,
	// which means the node is available
	return common.NodeAvailable, nil
}
