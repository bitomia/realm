package loads

type LoadsRepository interface {
	CreateLoad(loadName string, pid int, driver LoadDriver) error
	GetLoad(loadName string) (*Load, error)
	DeleteLoad(loadName string) error
}
