package drivers

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/bitomia/realm/internal"
)

type ProcessConfig struct {
	Name       string
	Node       string   `mapstructure:"node"`
	DependsOn  []string `mapstructure:"depends_on"`
	StartCmd   string   `mapstructure:"start_cmd"`
	StartArgs  *string  `mapstructure:"start_args,omitempty"`
	WorkingDir *string  `mapstructure:"working_dir,omitempty"`
	StopSignal string   `mapstructure:"stop_signal"`
}

type ProcessRequest struct {
	StartCmd   string  `json:"start_cmd"`
	StartArgs  *string `json:"start_args,omitempty"`
	WorkingDir *string `json:"working_dir,omitempty"`
	StopSignal string  `json:"stop_signal"`
}

type ProcessDriver struct {
	StartCmd   string
	StartArgs  *string
	WorkingDir *string
	StopSignal int
}

func NewProcessDriverFromConfig(config ProcessConfig) (LoadDriver, error) {
	stopSignal, ok := internal.StringToSignal(config.StopSignal)
	if !ok {
		return nil, fmt.Errorf("Invalid StopSignal")
	}

	driver := &ProcessDriver{
		StartCmd:   config.StartCmd,
		StartArgs:  config.StartArgs,
		WorkingDir: config.WorkingDir,
		StopSignal: stopSignal,
	}
	if err := driver.Verify(); err != nil {
		return nil, err
	}
	return driver, nil
}

func (p *ProcessDriver) GetDriverType() LoadDriverType {
	return ProcessDriverType
}

func (p *ProcessDriver) MarshalJSON() ([]byte, error) {
	return json.Marshal(&ProcessRequest{
		StartCmd:   p.StartCmd,
		StartArgs:  p.StartArgs,
		WorkingDir: p.WorkingDir,
		StopSignal: internal.SignalToString(p.StopSignal),
	})
}

func (p *ProcessDriver) UnmarshalJSON(data []byte) error {
	aux := &ProcessRequest{}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	p.StartCmd = aux.StartCmd
	p.StartArgs = aux.StartArgs
	p.WorkingDir = aux.WorkingDir

	stopSignal, ok := internal.StringToSignal(aux.StopSignal)
	if !ok {
		return fmt.Errorf("invalid stop signal: %s", aux.StopSignal)
	}
	p.StopSignal = stopSignal

	return nil
}

func (p *ProcessDriver) Verify() error {
	if strings.Contains(p.StartCmd, " ") {
		return fmt.Errorf("StartCmd must be one string")
	}
	return nil
}

func (p *ProcessDriver) VerifyDaemon() error {
	// Check StartCmd exists and it is executable
	if _, err := exec.LookPath(p.StartCmd); err != nil {
		return fmt.Errorf("Executable %q not found in PATH\n", p.StartCmd)
	}
	// Check WorkingDir exists
	if p.WorkingDir != nil {
		err := internal.DirExists(*p.WorkingDir)
		fmt.Printf("%v\n", err)
		if err := internal.DirExists(*p.WorkingDir); err != nil {
			return err
		}
	}
	return nil
}
