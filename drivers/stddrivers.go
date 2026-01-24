package drivers

import (
	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/drivers/loads"
	"github.com/bitomia/realm/drivers/nodes"
)

func RegisterStdDrivers() {
	common.RegisterLoadDriver(loads.ContainerDriver{})
	common.RegisterLoadDriver(loads.ProcessDriver{})
	common.RegisterLoadDriver(loads.QemuDriver{})
	common.RegisterNodeDriver(nodes.LinuxDriver{})
}
