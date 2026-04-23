package drivers

import (
	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/drivers/loads"
	"github.com/bitomia/realm/drivers/nodes"
)

func RegisterStdDrivers() error {
	if err := common.RegisterLoadDriver(&loads.ContainerDriver{}); err != nil {
		return err
	}
	if err := common.RegisterLoadDriver(&loads.ProcessDriver{}); err != nil {
		return err
	}
	if err := common.RegisterNodeDriver(&nodes.LinuxDriver{}); err != nil {
		return err
	}
	if err := common.RegisterNodeDriver(&nodes.WindowsDriver{}); err != nil {
		return err
	}
	return common.RegisterNodeDriver(&nodes.VMDriver{})
}
