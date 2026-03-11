package loads

import (
	"fmt"
	"runtime"
	"strings"

	process "github.com/shirou/gopsutil/v4/process"
)

func retrieveProcessByName(name string) (*process.Process, error) {
	procs, err := process.Processes()
	if err != nil {
		return nil, fmt.Errorf("failed to list processes: %w", err)
	}

	for _, p := range procs {
		pName, err := p.Name()
		if err != nil {
			continue
		}

		if (runtime.GOOS == "windows" && strings.EqualFold(pName, name)) || pName == name {
			return p, nil
		}
	}

	return nil, nil
}

func (p *ProcessDriver) shallUseProcessName() bool {
	return p.Config.UseProcessName != nil && *p.Config.UseProcessName
}
