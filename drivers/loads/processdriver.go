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
	configPkg "github.com/bitomia/realm/common/config"
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

func (p ProcessDriver) PlanAndRegister(repository common.DeploymentsRepository, loadName string) (common.DeploymentID, error) {
	// Check StartCmd exists and it is executable
	if _, err := exec.LookPath(p.Config.StartCmd); err != nil {
		return uuid.Nil, fmt.Errorf("Executable %q not found in PATH\n", p.Config.StartCmd)
	}

	// Check WorkingDir exists
	if p.Config.WorkingDir != nil {
		if err := internal.DirExists(*p.Config.WorkingDir); err != nil {
			return uuid.Nil, err
		}
	}

	// Check for existing running deployments (only running ones should block)
	deployments, err := repository.GetByLoadAndState(loadName, common.DeploymentStateRunning)
	if err != nil {
		slog.Error("ProcessDriver.PlanAndRegister", "msg", "Error on GetByLoadAndState", "error", err.Error())
		return uuid.Nil, err
	}
	if len(deployments) > 0 {
		return uuid.Nil, fmt.Errorf("Load for ProcessDriver already running: %s", loadName)
	}

	// Create deployment in "planned" state
	did, err := repository.Create(loadName, p, common.DeploymentStatePlanned, nil)
	if err != nil {
		slog.Error("ProcessDriver.PlanAndRegister", "msg", "failed to create deployment", "error", err)
		return uuid.Nil, err
	}

	return did, nil
}

func (p ProcessDriver) StartDeployment(repository common.DeploymentsRepository, deployment common.Deployment) error {
	// Verify deployment is in "planned" state
	if deployment.State != common.DeploymentStatePlanned {
		return fmt.Errorf("deployment %s is not in planned state", deployment.ID)
	}

	var args []string
	if p.Config.StartArgs != nil {
		args = strings.Fields(*p.Config.StartArgs)
	}

	cmd := exec.Command(p.Config.StartCmd, args...)

	if p.Config.WorkingDir != nil {
		cmd.Dir = *p.Config.WorkingDir
	}

	config := configPkg.Get()
	if config == nil {
		return fmt.Errorf("Cannot retrieve config")
	}
	logsPath := config.Daemon.LogsPath

	outfile, err := common.CreateLogFile(logsPath, fmt.Sprintf("%s.log", deployment.LoadName), 0755)
	if err != nil {
		return fmt.Errorf("Failed to create output log file: %v", err)
	}

	errfile, err := common.CreateLogFile(logsPath, fmt.Sprintf("%s_error.log", deployment.LoadName), 0755)
	if err != nil {
		return fmt.Errorf("Failed to create error log file: %v", err)
	}

	cmd.Env = os.Environ()
	cmd.Stdout = outfile
	cmd.Stderr = errfile

	if p.Config.WorkingDir != nil {
		cmd.Dir = *p.Config.WorkingDir
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start process: %w", err)
	}

	// Store process info
	procStore[deployment.ID] = ProcInfo{Cmd: cmd}

	// Update deployment state to "running"
	if err := repository.UpdateState(deployment.ID, common.DeploymentStateRunning); err != nil {
		return err
	}

	return nil
}

func (p ProcessDriver) StopDeployment(repository common.DeploymentsRepository, deployment common.Deployment) error {
	// Verify deployment is in "running" state
	if deployment.State != common.DeploymentStateRunning {
		return fmt.Errorf("deployment %s is not in running state", deployment.ID)
	}

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
		slog.Info("ProcessDriver.StopDeployment", "msg", "process exited", "pid", cmd.Process.Pid)
		if err := repository.UpdateState(deployment.ID, common.DeploymentStatePlanned); err != nil {
			return fmt.Errorf("failed to update load state: %w", err)
		}

	case <-time.After(3 * time.Second):
		slog.Error("ProcessDriver.StopDeployment", "msg", "process exit timeout", "pid", cmd.Process.Pid)
		if forceKill {
			slog.Info("ProcessDriver.StopDeployment", "msg", "force killing after timeout error", "pid", cmd.Process.Pid)
			cmd.Process.Kill()
		}
		<-done
	}

	// Clean up procStore
	delete(procStore, deployment.ID)

	return nil
}

func (p ProcessDriver) UnplanDeployment(repository common.DeploymentsRepository, deployment common.Deployment) error {
	// Verify deployment is in "planned" state
	if deployment.State != common.DeploymentStatePlanned {
		return fmt.Errorf("deployment %s is not in planned state", deployment.ID)
	}

	slog.Info("ProcessDriver.UnplanDeployment", "msg", "removing planned deployment", "deployment", deployment.ID)

	// For processes, there's nothing to clean up at unplan time
	// (no process started yet)
	if err := repository.DeleteDeployment(deployment.ID); err != nil {
		slog.Error("ProcessDriver.UnplanDeployment", "msg", "failed to delete deployment", "deploymentID", deployment.ID, "error", err)
		return fmt.Errorf("failed to delete deployment: %w", err)
	}

	return nil
}

func (p ProcessDriver) GetDriverConfig() common.LoadDriverConfig {
	return common.LoadDriverConfig{Driver: ProcessDriverID, DriverConfig: p.Config}
}
