//go:build exclude

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
	config HetznerServerConfig
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

func (*HetznerServerDriver) ID() common.NodeDriverID {
	return HetznerServerDriverID
}

func (*HetznerServerDriver) Info() (common.NodeDriverInfo, error) {
	return common.NewNodeDriverInfo(
		HetznerServerDriverID,
		NewHetznerServerDriverFromConfig,
		common.WithGuestMode(),
	)
}

func (d *HetznerServerDriver) Config() common.NodeDriverConfig {
	var c any = d.Config
	return common.NodeDriverConfig{Driver: HetznerServerDriverID, DriverConfig: &c}
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
