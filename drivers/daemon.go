package drivers

import (
	"fmt"
	"reflect"
)

type DaemonDriver interface {
	Description() string
}

var daemonDrivers []*DaemonDriver

func RegisterDaemonDriver(d DaemonDriver) error {
	for _, driver := range daemonDrivers {
		if reflect.TypeOf(driver) == reflect.TypeOf(d) {
			return fmt.Errorf("Daemon driver %T already registered", d)
		}
	}
	daemonDrivers = append(daemonDrivers, &d)
	return nil
}
