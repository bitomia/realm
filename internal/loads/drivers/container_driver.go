package drivers

import (
	"encoding/json"
	"fmt"

	"github.com/go-viper/mapstructure/v2"

	"github.com/bitomia/realm/internal"
	"github.com/bitomia/realm/internal/loads"
)

const ContainerDriverID loads.LoadDriverID = "container"

type ContainerDriver struct {
	Image string `json:"image"`
}

func NewContainerDriverFromConfig(c map[string]interface{}) (loads.LoadDriver, error) {
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

func (c ContainerDriver) DriverInfo() loads.LoadDriverInfo {
	return loads.LoadDriverInfo{
		ID:  ContainerDriverID,
		New: NewContainerDriverFromConfig,
	}
}

func (c ContainerDriver) GetLoadDriverID() loads.LoadDriverID {
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

func (c ContainerDriver) StartOnDaemon(repository loads.LoadsRepository, logsPath internal.LogsPath, loadName string) error {
	// TODO
	return fmt.Errorf("To be implemented")
}

func (c ContainerDriver) StopOnDaemon(repository loads.LoadsRepository, loadName string) error {
	// TODO
	return fmt.Errorf("To be implemented")
}
