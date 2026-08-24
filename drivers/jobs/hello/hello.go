package hello

import "github.com/bitomia/realm/common"

const HelloDriverID common.JobDriverID = "hello"

type HelloDriver struct{}

func (h *HelloDriver) ID() common.JobDriverID {
	return HelloDriverID
}

func (h *HelloDriver) Info() common.JobDriverInfo {
	return common.JobDriverInfo{
		ID: HelloDriverID,
		New: func(config any) (common.JobDriver, error) {
			return &HelloDriver{}, nil
		},
	}
}

func (h *HelloDriver) Run(args ...string) (*string, error) {
	msg := "hello world"
	return &msg, nil
}

func (h *HelloDriver) Config() common.JobDriverConfig {
	return common.JobDriverConfig{Driver: HelloDriverID, DriverConfig: nil}
}

func init() {
	_ = common.RegisterJobDriver(&HelloDriver{})
}
