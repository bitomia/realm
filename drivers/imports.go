package drivers

import (
	_ "github.com/bitomia/realm/drivers/jobs/hello"
	_ "github.com/bitomia/realm/drivers/loads/container"
	_ "github.com/bitomia/realm/drivers/loads/process"
	_ "github.com/bitomia/realm/drivers/nodes/linux"
	_ "github.com/bitomia/realm/drivers/nodes/vm"
	_ "github.com/bitomia/realm/drivers/nodes/windows"
)
