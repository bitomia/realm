package drivers

type DBLoads interface {
	CreateLoadEntry(processName string, pid int, driver LoadDriver) error
}
