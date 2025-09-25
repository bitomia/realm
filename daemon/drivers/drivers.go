package drivers

type Driver interface {
	Description() string
}

var drivers []*Driver

func RegisterDriver(d Driver) {
	// TODO make drivers unique
	drivers = append(drivers, &d)
}
