package drivers

import (
	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/drivers/loads"
)

func RegisterStdDrivers() {
	common.RegisterLoadDriver(loads.ContainerDriver{})
	common.RegisterLoadDriver(loads.ProcessDriver{})
}
