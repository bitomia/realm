package loads

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/google/uuid"

	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/internal"
)

const ProcessDriverID common.LoadDriverID = "process"

type ProcessConfig struct {
	Name       string
	Node       string   `mapstructure:"node"`
	DependsOn  []string `mapstructure:"depends_on"`
	StartCmd   string   `mapstructure:"start_cmd"`
	StartArgs  *string  `mapstructure:"start_args,omitempty"`
	WorkingDir *string  `mapstructure:"working_dir,omitempty"`
	StopSignal string   `mapstructure:"stop_signal"`
	ForceKill  bool     `mapstructure:"force_kill"`
}

type ProcessDriver struct {
	StartCmd   string
	StartArgs  *string
	WorkingDir *string
	StopSignal int
	ForceKill  bool
	LogsPath   common.LogsPath
	config     common.LoadDriverConfig
}

type ProcInfo struct {
	Cmd *exec.Cmd
}

var procStore map[common.DeploymentID]ProcInfo = make(map[common.DeploymentID]ProcInfo)

func NewProcessDriverFromConfig(c any) (common.LoadDriver, error) {
	var config ProcessConfig
	if err := mapstructure.Decode(c, &config); err != nil {
		return nil, err
	}

	stopSignal, ok := internal.StringToSignal(config.StopSignal)
	if !ok {
		return nil, fmt.Errorf("Invalid StopSignal")
	}

	driver := &ProcessDriver{
		StartCmd:   config.StartCmd,
		StartArgs:  config.StartArgs,
		WorkingDir: config.WorkingDir,
		StopSignal: stopSignal,
		ForceKill:  config.ForceKill,
		config:     common.LoadDriverConfig{Driver: ProcessDriverID, DriverConfig: c},
	}

	if err := driver.Verify(); err != nil {
		return nil, err
	}
	return driver, nil
}

func (c ProcessDriver) DriverInfo() common.LoadDriverInfo {
	return common.LoadDriverInfo{
		ID:  ProcessDriverID,
		New: NewProcessDriverFromConfig,
	}
}

func (c ProcessDriver) GetLoadDriverID() common.LoadDriverID {
	return ProcessDriverID
}

func (p ProcessDriver) MarshalJSON() ([]byte, error) {
	return json.Marshal(&p.config)
}

func (p ProcessDriver) UnmarshalJSON(data []byte) error {
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	if loadDriver, err := NewProcessDriverFromConfig(config); err != nil {
		return err
	} else {
		p = loadDriver.(ProcessDriver)
		return nil
	}
}

func (p ProcessDriver) Verify() error {
	if p.StartCmd == "" {
		return fmt.Errorf("StartCmd not specified")
	}
	if strings.Contains(p.StartCmd, " ") {
		return fmt.Errorf("StartCmd shall not have arguments")
	}
	if p.StopSignal == 0 {
		return fmt.Errorf("StopSignal not specified")
	}
	return nil
}

func (p ProcessDriver) PlanDaemon(repository common.DeploymentsRepository, loadName string) error {
	// Check StartCmd exists and it is executable
	if _, err := exec.LookPath(p.StartCmd); err != nil {
		return fmt.Errorf("Executable %q not found in PATH\n", p.StartCmd)
	}

	// Check WorkingDir exists
	if p.WorkingDir != nil {
		if err := internal.DirExists(*p.WorkingDir); err != nil {
			return err
		}
	}

	deployments, err := repository.GetByLoad(loadName)
	if err != nil {
		slog.Error("ProcessDriver.PlanDaemon", "msg", "Error on GetByLoad", "error", err.Error())
		return err
	}
	if len(deployments) > 0 {
		return fmt.Errorf("Load for ProcessDriver already active: %s", loadName)
	}

	return nil
}

func (p ProcessDriver) StartOnDaemon(repository common.DeploymentsRepository, logsPath common.LogsPath, loadName string) (common.DeploymentID, error) {
	var args []string
	if p.StartArgs != nil {
		args = strings.Fields(*p.StartArgs)
	}

	cmd := exec.Command(p.StartCmd, args...)

	if p.WorkingDir != nil {
		cmd.Dir = *p.WorkingDir
	}

	outfile, err := common.CreateLogFile(logsPath, fmt.Sprintf("%s.log", loadName), 0755)
	if err != nil {
		return uuid.Nil, fmt.Errorf("Failed to create output log file: %v", err)
	}

	errfile, err := common.CreateLogFile(logsPath, fmt.Sprintf("%s_error.log", loadName), 0755)
	if err != nil {
		return uuid.Nil, fmt.Errorf("Failed to create error log file: %v", err)
	}

	cmd.Env = os.Environ()
	cmd.Stdout = outfile
	cmd.Stderr = errfile

	if p.WorkingDir != nil {
		cmd.Dir = *p.WorkingDir
	}
	if err := cmd.Start(); err != nil {
		return uuid.Nil, fmt.Errorf("failed to start process: %w", err)
	}

	did, err := repository.Create(loadName, p)
	if err != nil {
		return uuid.Nil, err
	}
	procStore[did] = ProcInfo{Cmd: cmd}

	return did, nil
}

func (p ProcessDriver) StopOnDaemon(repository common.DeploymentsRepository, deployment common.Deployment) error {
	signal := internal.IntToSyscallSignal(p.StopSignal)

	forceKill := p.ForceKill
	cmd := procStore[deployment.ID].Cmd
	if err := cmd.Process.Signal(signal); err != nil {
		return fmt.Errorf("failed to send signal to process with PID %d: %w", cmd.Process.Pid, err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-done:
		slog.Info("ProcessDriver.StopOnDaemon", "msg", "process exited", "pid", cmd.Process.Pid)
		if err := repository.DeleteDeployment(deployment.ID); err != nil {
			return fmt.Errorf("failed to delete load from repository: %w", err)
		}

	case <-time.After(3 * time.Second):
		slog.Error("ProcessDriver.StopOnDaemon", "msg", "process exit timeout", "pid", cmd.Process.Pid)
		if forceKill {
			slog.Info("ProcessDriver.StopOnDaemon", "msg", "force killing after timeout error", "pid", cmd.Process.Pid)
			cmd.Process.Kill()
		}
		<-done
	}

	return nil
}

func (p ProcessDriver) GetDriverConfig() common.LoadDriverConfig {
	return p.config
}
