package drivers

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/bitomia/realm/internal"
	"github.com/bitomia/realm/internal/fs"
	"github.com/bitomia/realm/internal/signals"
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
	LogsPath   internal.LogsPath
}

func NewProcessDriverFromConfig(config ProcessConfig) (LoadDriver, error) {
	stopSignal, ok := signals.StringToSignal(config.StopSignal)
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
		StopSignal: signals.SignalToString(p.StopSignal),
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

	stopSignal, ok := signals.StringToSignal(aux.StopSignal)
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
		if err := fs.DirExists(*p.WorkingDir); err != nil {
			return err
		}
	}
	return nil
}

func (p *ProcessDriver) StartOnDaemon(logsPath internal.LogsPath, loadName string) error {
	var args []string
	if p.StartArgs != nil {
		args = strings.Fields(*p.StartArgs)
	}

	cmd := exec.Command(p.StartCmd, args...)

	if p.WorkingDir != nil {
		cmd.Dir = *p.WorkingDir
	}

	outfile, err := internal.CreateLogFile(logsPath, fmt.Sprintf("%s.log", loadName), 0755)
	if err != nil {
		return fmt.Errorf("Failed to create output log file: %v", err)
	}

	errfile, err := internal.CreateLogFile(logsPath, fmt.Sprintf("%s_error.log", loadName), 0755)
	if err != nil {
		return fmt.Errorf("Failed to create error log file: %v", err)
	}

	cmd.Env = os.Environ()
	cmd.Stdout = outfile
	cmd.Stderr = errfile

	if p.WorkingDir != nil {
		cmd.Dir = *p.WorkingDir
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start process: %w", err)
	}

	//pid := cmd.Process.Pid
	//database := db.GetDB()
	//database.CreateProcess()

	// TODO write PID to database

	return nil
}
