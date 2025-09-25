package drivers

import (
	"fmt"
	"reflect"
)

type Driver interface {
	Description() string
}

var drivers []*Driver

func RegisterDriver(d Driver) error {
	for _, driver := range drivers {
		if reflect.TypeOf(driver) == reflect.TypeOf(d) {
			return fmt.Errorf("Driver %T already registered", d)
		}
	}
	drivers = append(drivers, &d)
	return nil
}
