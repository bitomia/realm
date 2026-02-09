package loads

import (
	"encoding/json"
	"fmt"
	"io"
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
	StartCmd   string  `json:"start_cmd"`
	StartArgs  *string `json:"start_args,omitempty"`
	WorkingDir *string `json:"working_dir,omitempty"`
	StopSignal string  `json:"stop_signal"`
	ForceKill  bool    `json:"force_kill"`
}

type ProcessDriver struct {
	StopSignal int
	Config     ProcessConfig
}

type ProcInfo struct {
	Cmd *exec.Cmd
}

type ProcessEntryMetadata struct {
	Pid        int    `json:"pid,omitempty"`
	StdoutPath string `json:"stdout_path,omitempty"`
	StderrPath string `json:"stderr_path,omitempty"`
}

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

func (p ProcessDriver) PlanDeployment(repository common.DeploymentsRepository, loadName string) (common.DeploymentID, error) {
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
	deployments, err := repository.GetByLoadAndStatus(loadName, common.DeploymentStatusRunning)
	if err != nil {
		slog.Error("ProcessDriver.PlanDeployment", "msg", "Error on GetByLoadAndStatus", "error", err.Error())
		return uuid.Nil, err
	}
	if len(deployments) > 0 {
		return uuid.Nil, fmt.Errorf("Load for ProcessDriver already running: %s", loadName)
	}

	// Create deployment in "planned" state
	did, err := repository.Create(loadName, p, common.DeploymentStatus{StatusCode: common.DeploymentStatusPlanned}, ProcessEntryMetadata{})
	if err != nil {
		slog.Error("ProcessDriver.PlanDeployment", "msg", "failed to create deployment", "error", err)
		return uuid.Nil, err
	}

	return did, nil
}

func (p ProcessDriver) RunDeployment(repository common.DeploymentsRepository, deployment common.Deployment) error {
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
		return common.SetDeploymentError(repository, deployment, "ProcessDriver.RunDeployment", "deployment", deployment.ID, "error", "cannot retrieve config")
	}
	logsPath := config.Daemon.LogsPath

	stdoutPath := fmt.Sprintf("%s/%s_stdout.log", logsPath, deployment.LoadName)
	stderrPath := fmt.Sprintf("%s/%s_stderr.log", logsPath, deployment.LoadName)

	outfile, err := common.CreateLogFile(logsPath, fmt.Sprintf("%s_stdout.log", deployment.LoadName), 0755)
	if err != nil {
		return common.SetDeploymentError(repository, deployment, "ProcessDriver.RunDeployment", "deployment", deployment.ID, "error", fmt.Sprintf("Failed to create output log file: %v", err))
	}

	errfile, err := common.CreateLogFile(logsPath, fmt.Sprintf("%s_stderr.log", deployment.LoadName), 0755)
	if err != nil {
		return common.SetDeploymentError(repository, deployment, "ProcessDriver.RunDeployment", "deployment", deployment.ID, "error", fmt.Sprintf("Failed to create error log file: %v", err))
	}

	cmd.Env = os.Environ()
	cmd.Stdout = outfile
	cmd.Stderr = errfile

	if p.Config.WorkingDir != nil {
		cmd.Dir = *p.Config.WorkingDir
	}
	if err := cmd.Start(); err != nil {
		return common.SetDeploymentError(repository, deployment, "ProcessDriver.RunDeployment", "deployment", deployment.ID, "error", fmt.Sprintf("Failed to start process: %v", err))
	}

	// Update metadata with PID and file paths
	if err := common.UpdateMetadata(repository, deployment.ID, func(metadata *ProcessEntryMetadata) error {
		metadata.Pid = cmd.Process.Pid
		metadata.StdoutPath = stdoutPath
		metadata.StderrPath = stderrPath
		return nil
	}); err != nil {
		return common.SetDeploymentError(repository, deployment, "ProcessDriver.RunDeployment", "deployment", deployment.ID, "error", fmt.Sprintf("Failed to update metadata: %v", err))
	}

	if err := repository.UpdateStatus(deployment.ID, common.DeploymentStatus{StatusCode: common.DeploymentStatusRunning}); err != nil {
		return err
	}

	return nil
}

func (p ProcessDriver) StopDeployment(repository common.DeploymentsRepository, deployment common.Deployment) error {
	signal := internal.IntToSyscallSignal(p.StopSignal)

	forceKill := p.Config.ForceKill

	procInfo, err := retrieveProcessInfo(deployment)
	if err != nil {
		return common.SetDeploymentError(repository, deployment, "ProcessDriver.StopDeployment", "deployment", deployment.ID, "error", fmt.Errorf("process info not found in store for deployment %s", deployment.ID))
	}

	cmd := procInfo.Cmd
	if cmd.Process == nil {
		return common.SetDeploymentError(repository, deployment, "ProcessDriver.StopDeployment", "deployment", deployment.ID, "error", fmt.Errorf("process handle is nil for deployment %s", deployment.ID))
	}

	if err := cmd.Process.Signal(signal); err != nil {
		return common.SetDeploymentError(repository, deployment, "ProcessDriver.StopDeployment", "deployment", deployment.ID, "error", fmt.Errorf("failed to send signal to process with PID %d: %w", cmd.Process.Pid, err))
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-done:
		slog.Info("ProcessDriver.StopDeployment", "msg", "process exited", "pid", cmd.Process.Pid)
		if err := repository.UpdateStatus(deployment.ID, common.DeploymentStatus{StatusCode: common.DeploymentStatusStopped}); err != nil {
			return fmt.Errorf("failed to update load state: %w", err)
		}

	case <-time.After(3 * time.Second):
		slog.Error("ProcessDriver.StopDeployment", "msg", "process exit timeout", "pid", cmd.Process.Pid)
		if forceKill {
			slog.Info("ProcessDriver.StopDeployment", "msg", "force killing after timeout error", "pid", cmd.Process.Pid)

			cmd.Process.Kill()
			if err := repository.UpdateStatus(deployment.ID, common.DeploymentStatus{StatusCode: common.DeploymentStatusStopped}); err != nil {
				return fmt.Errorf("failed to update load state after force kill: %w", err)
			}
		}
		<-done
	}

	return nil
}

func (p ProcessDriver) UnplanDeployment(repository common.DeploymentsRepository, deployment common.Deployment) error {
	slog.Info("ProcessDriver.UnplanDeployment", "msg", "removing planned deployment", "deployment", deployment.ID)

	if deployment.Status.StatusCode == common.DeploymentStatusError {
		procInfo, _ := retrieveProcessInfo(deployment)
		if procInfo.Cmd != nil {
			procInfo.Cmd.Process.Kill()
		}
	}

	if err := repository.DeleteDeployment(deployment.ID); err != nil {
		return common.SetDeploymentError(repository, deployment, "ProcessDriver.StopDeployment", "deployment", deployment.ID, "error", fmt.Errorf("failed to delete deployment: %w", err))
	}

	return nil
}

func (p ProcessDriver) GetDriverConfig() common.LoadDriverConfig {
	return common.LoadDriverConfig{Driver: ProcessDriverID, DriverConfig: p.Config}
}

func (p ProcessDriver) UpdateDeploymentStatus(r common.DeploymentsRepository, d common.Deployment) (common.DeploymentStatus, error) {
	status := d.Status

	// Keep on error if it was on error status before
	if d.Status.StatusCode == common.DeploymentStatusError {
		return status, nil
	}

	procInfo, err := retrieveProcessInfo(d)
	if procInfo.Cmd == nil {
		if err != nil {
			// Only log error and continue to update as stopped
			slog.Error("ProcessDriver.UpdateDeploymentStatus", "msg", "retrieveProcessInfo failed", "error", err)
		}

		// At this point, it is planned, running or stopped
		// so it should transition only to stop if it was running
		if d.Status.StatusCode == common.DeploymentStatusRunning {
			status.StatusCode = common.DeploymentStatusStopped
		}
	} else {
		status.StatusCode = common.DeploymentStatusRunning
	}

	if err := r.UpdateStatus(d.ID, status); err != nil {
		slog.Error("ProcessDriver.UpdateDeploymentStatus", "msg", "failed to update deployment status", "error", err)
		return common.DeploymentStatus{}, err
	}

	return status, nil
}

func (p ProcessDriver) StreamStdout(repository common.DeploymentsRepository, deployment common.Deployment, w io.Writer) error {
	metadata, err := getProcessMetadata(deployment)
	if err != nil {
		return err
	}

	if len(metadata.StdoutPath) == 0 {
		return fmt.Errorf("stdout path empty")
	}

	return common.TailFile(metadata.StdoutPath, w)
}

func (p ProcessDriver) StreamStderr(repository common.DeploymentsRepository, deployment common.Deployment, w io.Writer) error {
	metadata, err := getProcessMetadata(deployment)
	if err != nil {
		return err
	}

	if len(metadata.StderrPath) == 0 {
		return fmt.Errorf("stderr path empty")
	}

	return common.TailFile(metadata.StderrPath, w)
}

func (p ProcessDriver) ReadStdout(repository common.DeploymentsRepository, deployment common.Deployment, offset int64) ([]byte, int64, error) {
	metadata, err := getProcessMetadata(deployment)
	if err != nil {
		return nil, 0, err
	}

	if len(metadata.StdoutPath) == 0 {
		return nil, 0, fmt.Errorf("stdout path empty")
	}

	return common.ReadFileAt(metadata.StdoutPath, offset)
}

func (p ProcessDriver) ReadStderr(repository common.DeploymentsRepository, deployment common.Deployment, offset int64) ([]byte, int64, error) {
	metadata, err := getProcessMetadata(deployment)
	if err != nil {
		return nil, 0, err
	}

	if len(metadata.StderrPath) == 0 {
		return nil, 0, fmt.Errorf("stderr path empty")
	}

	return common.ReadFileAt(metadata.StderrPath, offset)
}

func retrieveProcessInfo(deployment common.Deployment) (ProcInfo, error) {
	metadata, err := getProcessMetadata(deployment)
	if err != nil {
		return ProcInfo{nil}, nil
	}

	if metadata.Pid == 0 {
		return ProcInfo{nil}, nil
	}

	cmd, err := findProcess(metadata.Pid)
	return ProcInfo{cmd}, err
}

func getProcessMetadata(d common.Deployment) (*ProcessEntryMetadata, error) {
	var metadata ProcessEntryMetadata
	if tmp, err := json.Marshal(d.Metadata); err != nil {
		slog.Error("ProcessDriver.getMetadata", "error", "error on marshalling metadata", "deployment", d.ID)
		return nil, err
	} else {
		if err := json.Unmarshal(tmp, &metadata); err != nil {
			slog.Error("ProcessDriver.getMetadata", "error", "error on unmarshalling metadata", "deployment", d.ID)
			return nil, err
		}
	}
	return &metadata, nil
}
