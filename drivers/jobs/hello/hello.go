package hello

import (
	"fmt"
	"strconv"
	"time"

	"github.com/bitomia/realm/common"
)

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

func (h *HelloDriver) Run(w common.JobResultWriter, args ...string) error {
	times := 1
	if len(args) > 0 {
		parsed, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid repeat count %q: %v", args[0], err)
		}
		if parsed < 1 {
			return fmt.Errorf("repeat count must be at least 1, got %d", parsed)
		}
		times = parsed
	}

	for i := range times {
		if i > 0 {
			time.Sleep(time.Second)
		}
		if err := w.WriteValue("hello world"); err != nil {
			return err
		}
	}
	return nil
}

func (h *HelloDriver) Config() common.JobDriverConfig {
	return common.JobDriverConfig{Driver: HelloDriverID, DriverConfig: nil}
}

func init() {
	_ = common.RegisterJobDriver(&HelloDriver{})
}
