package drivers

import (
	"github.com/bitomia/realm/internal/loads"
	"github.com/bitomia/realm/internal/loads/drivers"
)

func RegisterStdDrivers() {
	loads.RegisterLoadDriver(drivers.ContainerDriver{})
	loads.RegisterLoadDriver(drivers.ProcessDriver{})
}
