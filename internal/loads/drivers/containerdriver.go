package drivers

import (
	"encoding/json"
	"fmt"
)

type ContainerConfig struct {
	Name      string
	Node      string   `mapstructure:"node"`
	DependsOn []string `mapstructure:"depends_on"`
	Image     string   `mapstructure:"image"`
}

type ContainerDriver struct {
	Image string `json:"image"`
}

func NewContainerDriverFromConfig(config ContainerConfig) (LoadDriver, error) {
	driver := &ContainerDriver{
		Image: config.Image,
	}
	if err := driver.Verify(); err != nil {
		return nil, err
	}
	return driver, nil
}

func (c *ContainerDriver) GetDriverType() LoadDriverType {
	return ContainerDriverType
}

func (p *ContainerDriver) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Image string `json:"image"`
	}{
		Image: p.Image,
	})
}

func (p *ContainerDriver) UnmarshalJSON(data []byte) error {
	aux := &struct {
		Image string `json:"image"`
	}{}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	p.Image = aux.Image
	return nil
}

func (p *ContainerDriver) Verify() error {
	return nil
}

func (p *ContainerDriver) VerifyDaemon() error {
	return nil
}

func (p *ContainerDriver) StartOnDaemon() error {
	// TODO
	return fmt.Errorf("To be implemneted")
}
