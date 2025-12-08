package loads

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/containerd/containerd"
	"github.com/go-viper/mapstructure/v2"
	"github.com/google/uuid"

	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/daemon/cruntime"
)

const ContainerDriverID common.LoadDriverID = "container"

type ContainerDriver struct {
	Image  string `json:"image"`
	config common.LoadDriverConfig
}

func NewContainerDriverFromConfig(c any) (common.LoadDriver, error) {
	var config ContainerDriver

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
	if err := decoder.Decode(c); err != nil {
		return nil, err
	}

	driver := &ContainerDriver{
		Image:  config.Image,
		config: common.LoadDriverConfig{Driver: ContainerDriverID, DriverConfig: c},
	}

	if err := driver.Verify(); err != nil {
		return nil, err
	}
	return driver, nil
}

func (c ContainerDriver) DriverInfo() common.LoadDriverInfo {
	return common.LoadDriverInfo{
		ID:  ContainerDriverID,
		New: NewContainerDriverFromConfig,
	}
}

func (c ContainerDriver) GetLoadDriverID() common.LoadDriverID {
	return ContainerDriverID
}

func (c ContainerDriver) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Image string `json:"image"`
	}{
		Image: c.Image,
	})
}

func (c ContainerDriver) UnmarshalJSON(data []byte) error {
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	if loadDriver, err := NewContainerDriverFromConfig(config); err != nil {
		return err
	} else {
		c = loadDriver.(ContainerDriver)
		return nil
	}

}

func (c ContainerDriver) Verify() error {
	if strings.Contains(c.Image, " ") {
		return fmt.Errorf("Container image not specified")
	}
	return nil
}

func (c ContainerDriver) PlanDaemon(repository common.DeploymentsRepository, loadName string) error {
	ctx, client, err := cruntime.CreateClient()
	if err != nil {
		slog.Error("ContainerDriver.PlanDaemon", "error", err)
		return err
	}
	defer client.Close()

	images, err := client.ImageService().List(ctx, fmt.Sprintf("name==%s", c.Image))
	if err != nil {
		slog.Error("ContainerDriver.PlanDaemon", "error", err)
		return err
	}
	if len(images) == 0 {
		slog.Info("ContainerDriver.PlanDaemon", "msg", "pulling image", "image", c.Image)
		image, err := client.Pull(ctx, c.Image, containerd.WithPullUnpack)
		if err != nil {
			slog.Error("ContainerDriver.PlanDaemon", "error", err)
			return err
		}
		slog.Info("ContainerDriver.PlanDaemon", "msg", "image pulled", "name", image.Name())
	}
	return nil
}

func (c ContainerDriver) StartOnDaemon(repository common.DeploymentsRepository, logsPath common.LogsPath, loadName string) (common.DeploymentID, error) {
	// TODO
	return uuid.Nil, fmt.Errorf("To be implemented")
}

func (c ContainerDriver) StopOnDaemon(repository common.DeploymentsRepository, deployment common.Deployment) error {
	// TODO
	return fmt.Errorf("To be implemented")
}

func (c ContainerDriver) GetDriverConfig() common.LoadDriverConfig {
	return c.config
}
