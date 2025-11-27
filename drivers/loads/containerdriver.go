package loads

import (
	"encoding/json"
	"fmt"

	"github.com/go-viper/mapstructure/v2"

	"github.com/bitomia/realm/config/logs"
)

const ContainerDriverID LoadDriverID = "container"

type ContainerDriver struct {
	Image string `json:"image"`
}

func NewContainerDriverFromConfig(c map[string]interface{}) (LoadDriver, error) {
	var config ContainerDriver
	if err := mapstructure.Decode(c, &config); err != nil {
		return nil, err
	}

	driver := &ContainerDriver{
		Image: config.Image,
	}

	if err := driver.Plan(); err != nil {
		return nil, err
	}
	return driver, nil
}

func (c ContainerDriver) DriverInfo() LoadDriverInfo {
	return LoadDriverInfo{
		ID:  ContainerDriverID,
		New: NewContainerDriverFromConfig,
	}
}

func (c ContainerDriver) GetLoadDriverID() LoadDriverID {
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

func (c ContainerDriver) Plan() error {
	return nil
}

func (c ContainerDriver) PlanDaemon() error {
	return nil
}

func (c ContainerDriver) StartOnDaemon(repository LoadsRepository, logsPath logs.LogsPath, loadName string) error {
	// TODO
	return fmt.Errorf("To be implemented")
}

func (c ContainerDriver) StopOnDaemon(repository LoadsRepository, loadName string) error {
	// TODO
	return fmt.Errorf("To be implemented")
}
