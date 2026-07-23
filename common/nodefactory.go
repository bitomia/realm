package common

import (
	"encoding/json"
	"fmt"
	"reflect"
)

type NodeDriverConfig struct {
	Driver       NodeDriverID `json:"driver"`
	DriverConfig *any         `json:"driver_config"`
}

// Equal compares two NodeDriverConfig by normalizing both DriverConfig
// values through JSON, since one side may hold a concrete driver struct
// and the other a map[string]any decoded from JSON.
func (d NodeDriverConfig) Equal(other NodeDriverConfig) bool {
	if d.Driver != other.Driver {
		return false
	}

	a, err := normalizeDriverConfig(d.DriverConfig)
	if err != nil {
		return false
	}
	b, err := normalizeDriverConfig(other.DriverConfig)
	if err != nil {
		return false
	}

	return reflect.DeepEqual(a, b)
}

func normalizeDriverConfig(c *any) (any, error) {
	data, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}

	var normalized any
	if err := json.Unmarshal(data, &normalized); err != nil {
		return nil, err
	}
	return normalized, nil
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
