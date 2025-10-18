package drivers

type LoadDriverType int

const (
	ContainerDriverType LoadDriverType = iota
	ProcessDriverType
)

const (
	ContainerDriverTypeStr string = "container"
	ProcessDriverTypeStr   string = "process"
)

var loadDriver = map[LoadDriverType]string{
	ContainerDriverType: ContainerDriverTypeStr,
	ProcessDriverType:   ProcessDriverTypeStr,
}

func (d LoadDriverType) String() string {
	return loadDriver[d]
}

type LoadDriver interface {
	GetDriverType() LoadDriverType
	Verify() error
	VerifyDaemon() error
	MarshalJSON() ([]byte, error)
	UnmarshalJSON(data []byte) error
	StartOnDaemon() error
}
