package boot

import "github.com/bitomia/realm/daemon/drivers"

type BootDriver interface {
	drivers.Driver

	Startup()
	Shutdown()
}
