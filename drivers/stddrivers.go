package drivers

import (
	"github.com/bitomia/realm/drivers/loads"
)

func RegisterStdDrivers() {
	loads.RegisterLoadDriver(loads.ContainerDriver{})
	loads.RegisterLoadDriver(loads.ProcessDriver{})
}
