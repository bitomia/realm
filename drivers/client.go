package drivers

import (
	"fmt"
	"reflect"
)

type ClientDriver interface {
	Description() string
}

var cliDrivers []*ClientDriver

func RegisterClientDriver(d ClientDriver) error {
	for _, driver := range cliDrivers {
		if reflect.TypeOf(driver) == reflect.TypeOf(d) {
			return fmt.Errorf("CLI Driver %T already registered", d)
		}
	}
	cliDrivers = append(cliDrivers, &d)
	return nil
}
