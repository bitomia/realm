package loads

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/containerd/containerd"
	"github.com/go-viper/mapstructure/v2"

	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/daemon/cruntime"
)

const ContainerDriverID common.LoadDriverID = "container"

type ContainerDriver struct {
	Image string `json:"image"`
}

func NewContainerDriverFromConfig(c map[string]interface{}) (common.LoadDriver, error) {
	var config ContainerDriver
	if err := mapstructure.Decode(c, &config); err != nil {
		return nil, err
	}

	driver := &ContainerDriver{
		Image: config.Image,
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
	aux := &struct {
		Image string `json:"image"`
	}{}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	c.Image = aux.Image

	return nil
}

func (c ContainerDriver) Verify() error {
	if strings.Contains(c.Image, " ") {
		return fmt.Errorf("Container image not specified")
	}
	return nil
}

func (c ContainerDriver) PlanDaemon() error {
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

func (c ContainerDriver) StartOnDaemon(repository common.LoadsRepository, logsPath common.LogsPath, loadName string) error {
	// TODO
	return fmt.Errorf("To be implemented")
}

func (c ContainerDriver) StopOnDaemon(repository common.LoadsRepository, loadName string) error {
	// TODO
	return fmt.Errorf("To be implemented")
}
