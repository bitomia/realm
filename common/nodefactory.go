package common

import (
	"fmt"
)

type NodeDriverConfig struct {
	Driver       NodeDriverID `json:"driver"`
	DriverConfig *any         `json:"driver_config"`
}

var nodeDrivers = make(map[NodeDriverID]NodeDriverInfo)

func RegisterNodeDriver(d NodeDriver) error {
	info, err := d.Info()
	if err != nil {
		return err
	}

	if _, exists := nodeDrivers[info.ID]; exists {
		return fmt.Errorf("nodeDriverID '%s' already registered", info.ID)
	}

	nodeDrivers[info.ID] = info
	return nil
}

func UnregisterNodeDriver(id NodeDriverID) error {
	if _, exists := nodeDrivers[id]; !exists {
		return fmt.Errorf("nodeDriverID '%s' not registered", id)
	}
	delete(nodeDrivers, id)
	return nil
}

func BuildNodeDriver(ctx NodeContext, d NodeDriverConfig) (NodeDriver, error) {
	if _, exists := nodeDrivers[d.Driver]; !exists {
		return nil, fmt.Errorf("nodeDriverID '%s' not registered", d.Driver)
	}

	driver, err := nodeDrivers[d.Driver].New(ctx, d.DriverConfig)
	if err != nil {
		return nil, err
	}

	return driver, nil
}
