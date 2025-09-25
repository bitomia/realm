package drivers

type BootDriver interface {
	Driver

	Startup()
	Shutdown()
}
