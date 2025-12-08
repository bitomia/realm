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
	Name       string   `json:"name"`
	Node       string   `json:"node"`
	DependsOn  []string `json:"depends_on"`
	StartCmd   string   `json:"start_cmd"`
	StartArgs  *string  `json:"start_args,omitempty"`
	WorkingDir *string  `json:"working_dir,omitempty"`
	StopSignal string   `json:"stop_signal"`
	ForceKill  bool     `json:"force_kill"`
}

type ProcessDriver struct {
	StopSignal int
	Config     ProcessConfig
}

type ProcInfo struct {
	Cmd *exec.Cmd
}

var procStore map[common.DeploymentID]ProcInfo = make(map[common.DeploymentID]ProcInfo)

func NewProcessDriverFromConfig(c any) (common.LoadDriver, error) {
	var config ProcessConfig

	// Configure mapstructure decoder to use 'json' tags
	// because it has to work for config files (yaml)
	// and for remote commands (json)
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		TagName: "json",
		Result:  &config,
	})
	if err != nil {
		return nil, err
	}
	if err := decoder.Decode(c); err != nil {
		return nil, err
	}

	stopSignal, ok := internal.StringToSignal(config.StopSignal)
	if !ok {
		return nil, fmt.Errorf("Invalid StopSignal")
	}

	driver := &ProcessDriver{
		StopSignal: stopSignal,
		Config:     config,
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
	return json.Marshal(p.GetDriverConfig())
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
	if p.Config.StartCmd == "" {
		return fmt.Errorf("StartCmd not specified")
	}
	if strings.Contains(p.Config.StartCmd, " ") {
		return fmt.Errorf("StartCmd shall not have arguments")
	}
	if p.StopSignal == 0 {
		return fmt.Errorf("StopSignal not specified")
	}
	return nil
}

func (p ProcessDriver) PlanDaemon(repository common.DeploymentsRepository, loadName string) error {
	// Check StartCmd exists and it is executable
	if _, err := exec.LookPath(p.Config.StartCmd); err != nil {
		return fmt.Errorf("Executable %q not found in PATH\n", p.Config.StartCmd)
	}

	// Check WorkingDir exists
	if p.Config.WorkingDir != nil {
		if err := internal.DirExists(*p.Config.WorkingDir); err != nil {
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
	if p.Config.StartArgs != nil {
		args = strings.Fields(*p.Config.StartArgs)
	}

	cmd := exec.Command(p.Config.StartCmd, args...)

	if p.Config.WorkingDir != nil {
		cmd.Dir = *p.Config.WorkingDir
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

	if p.Config.WorkingDir != nil {
		cmd.Dir = *p.Config.WorkingDir
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

	forceKill := p.Config.ForceKill
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
	return common.LoadDriverConfig{Driver: ProcessDriverID, DriverConfig: p.Config}
}
