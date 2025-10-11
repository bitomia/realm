package config

type ContainerDriver struct {
	Image string `json:"image"`
}

func NewContainerDriverFromConfig(config ContainerConfig) *ContainerDriver {
	return &ContainerDriver{
		Image: config.Image,
	}
}

func (c *ContainerDriver) GetDriverType() LoadDriverType {
	return ContainerDriverType
}
