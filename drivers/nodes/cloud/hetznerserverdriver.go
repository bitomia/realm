package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/go-viper/mapstructure/v2"
	"github.com/hetznercloud/hcloud-go/hcloud"

	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/common/cloudinit"
)

const (
	HetznerServerDriverID common.NodeDriverID = "hetzner_cloud_server"
	Application                               = "realm"
	ApplicationVersion                        = "v1.0.0"
)

type HetznerServerConfig struct {
	ServerType string `json:"type"`
	Location   string `json:"location"`
}

type HetznerServerDriver struct {
	Config HetznerServerConfig
	client *hcloud.Client
}

func NewHetznerServerDriverFromConfig(c *any) (common.NodeDriver, error) {
	if c == nil {
		return &HetznerServerDriver{}, fmt.Errorf("config cannot be nil")
	}

	var config HetznerServerConfig
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

	hetznerToken := os.Getenv("HETZNER_TOKEN")
	if hetznerToken == "" {
		return &HetznerServerDriver{}, fmt.Errorf("env var HETZNER_TOKEN required")
	}

	hetznerClient := hcloud.NewClient(
		hcloud.WithToken(hetznerToken),
		hcloud.WithApplication(Application, ApplicationVersion),
	)
	if hetznerClient == nil {
		return &HetznerServerDriver{}, fmt.Errorf("hetzner client creation failed")
	}

	return &HetznerServerDriver{config, hetznerClient}, nil
}

func (*HetznerServerDriver) GetNodeDriverID() common.NodeDriverID {
	return HetznerServerDriverID
}

func (*HetznerServerDriver) DriverInfo() (common.NodeDriverInfo, error) {
	return common.NewNodeDriverInfo(
		HetznerServerDriverID,
		NewHetznerServerDriverFromConfig,
		false,
		common.WithStartMode(common.ClientMode),
		common.WithStopMode(common.ClientMode),
	)
}

func (d *HetznerServerDriver) GetDriverConfig() common.NodeDriverConfig {
	var c any = d.Config
	return common.NodeDriverConfig{Driver: HetznerServerDriverID, DriverConfig: &c}
}

func (d *HetznerServerDriver) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.GetDriverConfig())
}

func (d *HetznerServerDriver) UnmarshalJSON(data []byte) error {
	var cfgMap map[string]any
	if err := json.Unmarshal(data, &cfgMap); err != nil {
		return err
	}

	var nodeDriver common.NodeDriver
	var err error
	if len(cfgMap) > 0 {
		var a any = cfgMap
		nodeDriver, err = NewHetznerServerDriverFromConfig(&a)
	} else {
		nodeDriver, err = NewHetznerServerDriverFromConfig(nil)
	}
	if err != nil {
		return err
	}
	*d = *nodeDriver.(*HetznerServerDriver)
	return nil

}

func (d *HetznerServerDriver) Start(nodeName *string, repository common.NodesRepository) error {
	return fmt.Errorf("not implemented")
}

func (d *HetznerServerDriver) Stop(nodeName *string, message string, time uint32, repository common.NodesRepository, force bool) error {
	return fmt.Errorf("not implemented")
}

func (d *HetznerServerDriver) Restart(nodeName *string, message string, time uint32, repository common.NodesRepository) error {
	return fmt.Errorf("not implemented")
}

func (d *HetznerServerDriver) UpdateStatus(nodeName *string, repository common.NodesRepository) (common.NodeStatus, error) {
	return common.NodeStatus{}, fmt.Errorf("not implemented")
}

func (d *HetznerServerDriver) GetState(nodeName *string, repository common.NodesRepository) (common.NodeState, error) {
	return common.NodeState{}, fmt.Errorf("not implemented")
}

func (d *HetznerServerDriver) Provision(nodeName string, cloudInit *cloudinit.CloudInit, repository common.NodesRepository) error {
	return fmt.Errorf("not implemented")
}

func (d *HetznerServerDriver) Deprovision(nodeName *string, repository common.NodesRepository) error {
	return fmt.Errorf("not implemented")
}
