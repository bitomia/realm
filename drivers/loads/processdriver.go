package loads

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"

	"github.com/go-viper/mapstructure/v2"
	"github.com/google/uuid"
	process "github.com/shirou/gopsutil/v4/process"

	"github.com/bitomia/realm/common"
	configPkg "github.com/bitomia/realm/daemon/config"
	"github.com/bitomia/realm/internal"
)

const ProcessDriverID common.LoadDriverID = "process"

type ProcessConfig struct {
	StartCmd       string  `json:"start_cmd"`
	StartArgs      *string `json:"start_args,omitempty"`
	WorkingDir     *string `json:"working_dir,omitempty"`
	StopSignal     *string `json:"stop_signal,omitempty"`
	UseProcessName *bool   `json:"use_process_name,omitempty"`
}

type ProcessDriver struct {
	StopSignal *int
	Config     ProcessConfig
}

type ProcessEntryMetadata struct {
	Pid        int    `json:"pid,omitempty"`
	StdoutPath string `json:"stdout_path,omitempty"`
	StderrPath string `json:"stderr_path,omitempty"`
}

func NewProcessDriver(c any) (common.LoadDriver, error) {
	var config = ProcessConfig{}

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

	var stopSignal *int = nil
	if config.StopSignal != nil {
		if stopSignalAux, ok := internal.StringToSignal(*config.StopSignal); !ok {
			return nil, fmt.Errorf("Invalid StopSignal")
		} else {
			stopSignal = &stopSignalAux
		}
	}

	driver := &ProcessDriver{
		StopSignal: stopSignal,
		Config:     config,
	}

	if err := driver.verifyConfig(); err != nil {
		return nil, err
	}
	return driver, nil
}

func (c ProcessDriver) DriverInfo() common.LoadDriverInfo {
	return common.LoadDriverInfo{
		ID:  ProcessDriverID,
		New: NewProcessDriver,
	}
}

func (c ProcessDriver) GetLoadDriverID() common.LoadDriverID {
	return ProcessDriverID
}

func (p *ProcessDriver) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.GetDriverConfig())
}

func (p *ProcessDriver) UnmarshalJSON(data []byte) error {
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	if loadDriver, err := NewProcessDriver(config); err != nil {
		return err
	} else {
		p = loadDriver.(*ProcessDriver)
		return nil
	}
}

func (p *ProcessDriver) verifyConfig() error {
	if p.Config.StartCmd == "" {
		return fmt.Errorf("StartCmd not specified")
	}

	return nil
}

func (p *ProcessDriver) Provision(nodeDriver common.NodeDriver, repository common.DeploymentsRepository, loadName string) (common.DeploymentID, error) {
	resolved, err := p.resolveStartCmdPath()
	if err != nil {
		return uuid.Nil, err
	}
	p.Config.StartCmd = resolved

	// Check WorkingDir exists
	if p.Config.WorkingDir != nil {
		if err := internal.DirExists(*p.Config.WorkingDir); err != nil {
			return uuid.Nil, err
		}
	}

	// Check for existing running deployments (only running ones should block)
	deployments, err := repository.GetByLoadAndStatus(loadName, common.DeploymentStatusRunning)
	if err != nil {
		slog.Error("ProcessDriver.Provision", "msg", "Error on GetByLoadAndStatus", "error", err.Error())
		return uuid.Nil, err
	}

	if len(deployments) > 0 {
		return uuid.Nil, fmt.Errorf("Load for ProcessDriver already running: %s", loadName)
	}

	// Create deployment in "provisioned" state
	did, err := repository.Create(loadName, p, common.DeploymentStatus{StatusCode: common.DeploymentStatusReady}, ProcessEntryMetadata{})
	if err != nil {
		slog.Error("ProcessDriver.Provision", "msg", "failed to create deployment", "error", err)
		return uuid.Nil, err
	}

	return did, nil
}

func (p *ProcessDriver) Run(repository common.DeploymentsRepository, deployment common.Deployment) error {
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
		return common.SetDeploymentError(repository, deployment, "ProcessDriver.Run", "deployment", deployment.ID, "error", "cannot retrieve config")
	}

	stdoutPath := filepath.Join(config.Daemon.DataPath, "logs", "ps", fmt.Sprintf("%s_stdout.log", deployment.LoadName))
	stderrPath := filepath.Join(config.Daemon.DataPath, "logs", "ps", fmt.Sprintf("%s_stderr.log", deployment.LoadName))

	stdoutFile, err := common.CreateLogFile(stdoutPath, 0755)
	if err != nil {
		return common.SetDeploymentError(repository, deployment, "ProcessDriver.Run", "deployment", deployment.ID, "error", fmt.Sprintf("Failed to create output log file: %v", err))
	}

	stderrFile, err := common.CreateLogFile(stderrPath, 0755)
	if err != nil {
		return common.SetDeploymentError(repository, deployment, "ProcessDriver.Run", "deployment", deployment.ID, "error", fmt.Sprintf("Failed to create error log file: %v", err))
	}

	cmd.Env = os.Environ()
	cmd.Stdout = stdoutFile
	cmd.Stderr = stderrFile

	if p.Config.WorkingDir != nil {
		cmd.Dir = *p.Config.WorkingDir
	}
	if err := cmd.Start(); err != nil {
		return common.SetDeploymentError(repository, deployment, "ProcessDriver.Run", "deployment", deployment.ID, "error", fmt.Sprintf("Failed to start process: %v", err))
	}

	slog.Info("ProcessDriver.Run", "msg", "process started", "deployment", deployment.ID, "pid", cmd.Process.Pid)

	// Update metadata with PID and file paths
	if err := common.UpdateDeploymentMetadata(repository, deployment.ID, func(metadata *ProcessEntryMetadata) error {
		metadata.Pid = cmd.Process.Pid
		metadata.StdoutPath = stdoutPath
		metadata.StderrPath = stderrPath
		return nil
	}); err != nil {
		return common.SetDeploymentError(repository, deployment, "ProcessDriver.Run", "deployment", deployment.ID, "error", fmt.Sprintf("Failed to update metadata: %v", err))
	}

	if err := repository.UpdateStatus(deployment.ID, common.DeploymentStatus{StatusCode: common.DeploymentStatusRunning}); err != nil {
		return err
	}

	return nil
}

func (p *ProcessDriver) Stop(repository common.DeploymentsRepository, deployment common.Deployment) error {
	var proc *process.Process
	var err error

	if p.shallUseProcessName() {
		name := filepath.Base(p.Config.StartCmd)
		proc, err = retrieveProcessByName(name)
	} else {
		proc, err = retrieveProcess(deployment)
	}

	if err != nil {
		return common.SetDeploymentError(repository, deployment, "ProcessDriver.Stop", "deployment", deployment.ID, "error", fmt.Errorf("process not found in store for deployment %s", deployment.ID))
	}

	if proc == nil {
		return common.SetDeploymentError(repository, deployment, "ProcessDriver.Stop", "deployment", deployment.ID, "error", fmt.Errorf("process handle is nil for deployment %s", deployment.ID))
	}

	slog.Info("ProcessDriver.Stop", "msg", "stopping process", "deployment", deployment.ID, "pid", proc.Pid)

	if err := stopProcess(proc, p.StopSignal); err != nil {
		return common.SetDeploymentError(repository, deployment, "ProcessDriver.Stop", "deployment", deployment.ID, "error", fmt.Errorf("failed to stop process with PID %d: %w", proc.Pid, err))
	}

	return nil
}

func (p *ProcessDriver) Kill(repository common.DeploymentsRepository, deployment common.Deployment) error {
	var proc *process.Process
	var err error

	if p.shallUseProcessName() {
		name := filepath.Base(p.Config.StartCmd)
		proc, err = retrieveProcessByName(name)
	} else {
		proc, err = retrieveProcess(deployment)
	}

	if err != nil {
		return common.SetDeploymentError(repository, deployment, "ProcessDriver.Kill", "deployment", deployment.ID, "error", fmt.Errorf("process not found in store for deployment %s", deployment.ID))
	}

	if proc == nil {
		return common.SetDeploymentError(repository, deployment, "ProcessDriver.Kill", "deployment", deployment.ID, "error", fmt.Errorf("process handle is nil for deployment %s", deployment.ID))
	}

	slog.Info("ProcessDriver.Kill", "msg", "killing process", "deployment", deployment.ID, "pid", proc.Pid)

	if err := proc.Kill(); err != nil {
		return common.SetDeploymentError(repository, deployment, "ProcessDriver.Kill", "deployment", deployment.ID, "error", fmt.Errorf("failed to send signal to process with PID %d: %w", proc.Pid, err))
	}

	return nil
}

func (p *ProcessDriver) Deprovision(repository common.DeploymentsRepository, deployment common.Deployment) error {
	slog.Info("ProcessDriver.Deprovision", "msg", "removing provisioned deployment", "deployment", deployment.ID)

	if deployment.Status.StatusCode == common.DeploymentStatusError {
		proc, _ := retrieveProcess(deployment)
		if proc != nil {
			proc.Kill()
		}
	}

	if err := repository.DeleteDeployment(deployment.ID); err != nil {
		return common.SetDeploymentError(repository, deployment, "ProcessDriver.Stop", "deployment", deployment.ID, "error", fmt.Errorf("failed to delete deployment: %w", err))
	}

	return nil
}

func (p *ProcessDriver) GetDriverConfig() common.LoadDriverConfig {
	return common.LoadDriverConfig{Driver: ProcessDriverID, DriverConfig: p.Config}
}

func (p *ProcessDriver) UpdateStatus(r common.DeploymentsRepository, d common.Deployment) (common.DeploymentStatus, error) {
	status := d.Status

	// Keep on error if it was on error status before
	if d.Status.StatusCode == common.DeploymentStatusError {
		return status, nil
	}

	var proc *process.Process
	var err error

	if p.shallUseProcessName() {
		name := filepath.Base(p.Config.StartCmd)
		proc, err = retrieveProcessByName(name)
	} else {
		proc, err = retrieveProcess(d)
	}

	if err == process.ErrorProcessNotRunning {
		status.StatusCode = common.DeploymentStatusStopped
	} else if proc == nil {
		if err != nil {
			// Only log error and continue to update as stopped
			slog.Warn("ProcessDriver.UpdateStatus", "msg", "retrieve process failed", "error", err)
			status.StatusCode = common.DeploymentStatusStopped
		} else {
			// At this point, it is provisioned, running or stopped
			// so it should transition only to stop if it was running
			if d.Status.StatusCode == common.DeploymentStatusRunning {
				status.StatusCode = common.DeploymentStatusStopped
			}
		}
	} else {
		// Process might be running or defunct
		// Status code is stopped if it's defunct
		if defunct, _ := isDefunct(proc); defunct == true {
			status.StatusCode = common.DeploymentStatusStopped
		} else {
			status.StatusCode = common.DeploymentStatusRunning
		}
	}

	if err := r.UpdateStatus(d.ID, status); err != nil {
		slog.Error("ProcessDriver.UpdateStatus", "msg", "failed to update deployment status", "error", err)
		return common.DeploymentStatus{}, err
	}

	return status, nil
}

func (p *ProcessDriver) StreamStdout(repository common.DeploymentsRepository, deployment common.Deployment, w io.Writer) error {
	metadata, err := getProcessMetadata(deployment)
	if err != nil {
		return err
	}

	if len(metadata.StdoutPath) == 0 {
		return fmt.Errorf("stdout path empty")
	}

	return common.TailFile(metadata.StdoutPath, w)
}

func (p *ProcessDriver) StreamStderr(repository common.DeploymentsRepository, deployment common.Deployment, w io.Writer) error {
	metadata, err := getProcessMetadata(deployment)
	if err != nil {
		return err
	}

	if len(metadata.StderrPath) == 0 {
		return fmt.Errorf("stderr path empty")
	}

	return common.TailFile(metadata.StderrPath, w)
}

func (p *ProcessDriver) ReadStdout(repository common.DeploymentsRepository, deployment common.Deployment, offset int64) ([]byte, int64, error) {
	metadata, err := getProcessMetadata(deployment)
	if err != nil {
		return nil, 0, err
	}

	if len(metadata.StdoutPath) == 0 {
		return nil, 0, fmt.Errorf("stdout path empty")
	}

	return common.ReadFileAt(metadata.StdoutPath, offset)
}

func (p *ProcessDriver) ReadStderr(repository common.DeploymentsRepository, deployment common.Deployment, offset int64) ([]byte, int64, error) {
	metadata, err := getProcessMetadata(deployment)
	if err != nil {
		return nil, 0, err
	}

	if len(metadata.StderrPath) == 0 {
		return nil, 0, fmt.Errorf("stderr path empty")
	}

	return common.ReadFileAt(metadata.StderrPath, offset)
}

// Resolve StartCmd by priority: absolute path, working directory, PATH env var
func (p *ProcessDriver) resolveStartCmdPath() (string, error) {
	if filepath.IsAbs(p.Config.StartCmd) {
		if _, err := os.Stat(p.Config.StartCmd); err == nil {
			return p.Config.StartCmd, nil
		}
	}
	if p.Config.WorkingDir != nil {
		workingDirCmdPath := filepath.Join(*p.Config.WorkingDir, p.Config.StartCmd)
		if _, err := os.Stat(workingDirCmdPath); err == nil {
			return workingDirCmdPath, nil
		}
	}
	if path, err := exec.LookPath(p.Config.StartCmd); err == nil {
		return path, nil
	}

	return "", fmt.Errorf("Executable %q not found", p.Config.StartCmd)
}

func retrieveProcess(deployment common.Deployment) (*process.Process, error) {
	metadata, err := getProcessMetadata(deployment)
	if err != nil {
		return nil, nil
	}
	if metadata.Pid == 0 {
		return nil, nil
	}

	p, err := os.FindProcess(metadata.Pid)
	if err != nil {
		return nil, err
	}

	if runtime.GOOS != "windows" {
		// On Unix systems, FindProcess always succeeds and returns a Process
		// for the given pid, regardless of whether the process exists. To test whether
		// the process actually exists, see whether p.Signal(syscall.Signal(0)) reports
		// an error.
		if err := p.Signal(syscall.Signal(0)); err != nil {
			return nil, err
		}
	}

	return process.NewProcess(int32(metadata.Pid))
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

func isDefunct(p *process.Process) (bool, error) {
	statuses, err := p.Status()
	if err != nil {
		return false, err
	}
	if slices.Contains(statuses, process.Zombie) {
		return true, nil
	}
	return false, nil
}
