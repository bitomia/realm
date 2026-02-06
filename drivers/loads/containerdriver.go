package loads

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"path"
	"syscall"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/oci"
	"github.com/go-viper/mapstructure/v2"
	"github.com/google/uuid"

	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/common/config"
	"github.com/bitomia/realm/common/dto"
	"github.com/bitomia/realm/daemon/api"
	"github.com/bitomia/realm/daemon/containers"
	"github.com/bitomia/realm/daemon/cruntime"
	"github.com/bitomia/realm/daemon/network"
)

const ContainerDriverID common.LoadDriverID = "container"

type ContainerConfig struct {
	Image       string             `json:"image"`
	Env         []string           `json:"env"`
	Quotas      *dto.Quotas        `json:"quotas"`
	MountVolume *[]dto.MountVolume `json:"mount_volume,omitempty"`
	BindMounts  []dto.BindMount    `json:"bind_mounts,omitempty"`
	Network     *dto.NetworkConfig `json:"network,omitempty"`
	Entrypoint  *string            `json:"entrypoint,omitempty"`
	Args        []string           `json:"args,omitempty"`
	WorkingDir  *string            `json:"working_dir,omitempty"`
}

type ContainerDriver struct {
	Config ContainerConfig
}

type ContainerEntryMetadata struct {
	ContainerName string  `json:"container_name"`
	IPAddress     *net.IP `json:"ip_address,omitempty"`
	GWAddress     *net.IP `json:"gw_address,omitempty"`
	StdoutPath    string  `json:"stdout_path,omitempty"`
	StderrPath    string  `json:"stderr_path,omitempty"`
}

func NewContainerDriverFromConfig(c any) (common.LoadDriver, error) {
	var config ContainerConfig

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

	driver := &ContainerDriver{
		Config: config,
	}

	if err := driver.Verify(); err != nil {
		return nil, err
	}
	return driver, nil
}

func (c ContainerDriver) DriverInfo() common.LoadDriverInfo {
	return common.LoadDriverInfo{
		ID:  ContainerDriverID,
		New: NewContainerDriverFromConfig,
	}
}

func (c ContainerDriver) GetLoadDriverID() common.LoadDriverID {
	return ContainerDriverID
}

func (c ContainerDriver) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.GetDriverConfig())
}

func (c ContainerDriver) UnmarshalJSON(data []byte) error {
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	if loadDriver, err := NewContainerDriverFromConfig(config); err != nil {
		return err
	} else {
		c = loadDriver.(ContainerDriver)
		return nil
	}
}

func (c ContainerDriver) Verify() error {
	if c.Config.Image == "" {
		return fmt.Errorf("Container image not specified")
	}

	// Validate network configuration
	if c.Config.Network != nil {
		mode := c.Config.Network.Mode
		if mode == "" {
			mode = "bridge" // default mode
		}

		// Validate mode is valid
		if mode != "bridge" && mode != "host" {
			return fmt.Errorf("Invalid network mode '%s': must be 'bridge' or 'host'", mode)
		}

		// If not using host mode, network name is required
		if mode != "host" && c.Config.Network.Network == "" {
			return fmt.Errorf("Network name is required when using bridge mode")
		}

		// Warn during planning if host mode has conflicting settings
		if mode == "host" {
			if len(c.Config.Network.PortMap) > 0 {
				slog.Warn("ContainerDriver.Verify", "msg", "port_map will be ignored in host network mode")
			}
			if c.Config.Network.IPMasq {
				slog.Warn("ContainerDriver.Verify", "msg", "ip_masq will be ignored in host network mode")
			}
			if c.Config.Network.DNS {
				slog.Warn("ContainerDriver.Verify", "msg", "dns will be ignored in host network mode")
			}
			if c.Config.Network.Network != "" {
				slog.Warn("ContainerDriver.Verify", "msg", "network name will be ignored in host network mode")
			}
		}
	}

	return nil
}

func (c ContainerDriver) PlanDeployment(repository common.DeploymentsRepository, loadName string) (common.DeploymentID, error) {
	ctx, client, err := cruntime.CreateClient()
	if err != nil {
		slog.Error("ContainerDriver.PlanAndRegister", "error", err)
		return uuid.Nil, err
	}
	defer client.Close()

	_, err = containers.TryPullAndGetImage(ctx, client, c.Config.Image)
	if err != nil {
		slog.Error("ContainerDriver.PlanAndRegister", "error", err)
		return uuid.Nil, err
	}

	did, err := repository.Create(loadName, c, common.DeploymentStatus{StatusCode: common.DeploymentStatusPlanned}, ContainerEntryMetadata{})
	if err != nil {
		slog.Error("ContainerDriver.PlanDeployment", "msg", "failed to create deployment", "error", err)
		return uuid.Nil, err
	}

	return did, nil
}

func (c ContainerDriver) UnplanDeployment(repository common.DeploymentsRepository, deployment common.Deployment) error {
	if deployment.Status.StatusCode != common.DeploymentStatusStopped && deployment.Status.StatusCode != common.DeploymentStatusError {
		return fmt.Errorf("deployment %s not in valid state", deployment.ID)
	}

	slog.Info("ContainerDriver.UnplanDeployment", "msg", "unplanning deployment", "deployment", deployment.ID)

	// Clean-up if error
	if deployment.Status.StatusCode == common.DeploymentStatusError {
		var metadata ContainerEntryMetadata
		if tmp, err := json.Marshal(deployment.Metadata); err != nil {
			slog.Warn("ContainerDriver.UnplanDeployment", "error", "error on retrieving metadata", "deployment", deployment.ID)
			goto unplan_deployment
		} else {
			json.Unmarshal(tmp, &metadata)
		}

		if len(metadata.ContainerName) > 0 {
			if err := c.cleanupContainer(metadata.ContainerName, syscall.SIGKILL, false); err != nil {
				slog.Warn("ContainerDriver.UnplanDeployment", "msg", "failed to clean-up container. possible orphan container", "deploymentID", deployment.ID, "container", metadata.ContainerName, "error", err)
			}
		}
	}

unplan_deployment:
	if err := repository.DeleteDeployment(deployment.ID); err != nil {
		slog.Error("ContainerDriver.UnplanDeployment", "msg", "failed to delete deployment", "deploymentID", deployment.ID, "error", err)

		return repository.UpdateStatus(deployment.ID, common.DeploymentStatus{StatusCode: common.DeploymentStatusError, Reason: fmt.Sprintf("failed to delete deployment: %s", err.Error())})
	}

	return nil
}

func (c ContainerDriver) RunDeployment(repository common.DeploymentsRepository, deployment common.Deployment) error {
	// Verify deployment is in "planned" state
	if deployment.Status.StatusCode != common.DeploymentStatusPlanned {
		return fmt.Errorf("deployment %s is not in planned status", deployment.ID)
	}

	// Use loadName to create a unique container name
	containerName := fmt.Sprintf("%s-%s", deployment.LoadName, uuid.New())
	slog.Info("ContainerDriver.RunDeployment", "msg", "starting container", "container", containerName)

	createOpts := dto.CreateContainerRequest{
		Image:       c.Config.Image,
		Quotas:      c.Config.Quotas,
		Env:         c.Config.Env,
		MountVolume: c.Config.MountVolume,
		BindMounts:  c.Config.BindMounts,
		Entrypoint:  c.Config.Entrypoint,
		Args:        c.Config.Args,
		WorkingDir:  c.Config.WorkingDir,
	}

	// Prepare extra OCI spec options for host networking if needed
	var extraSpecOpts []oci.SpecOpts
	if c.Config.Network != nil && c.Config.Network.Mode == "host" {
		slog.Info("ContainerDriver.RunDeployment", "msg", "using host network mode", "container", containerName)
		extraSpecOpts = containers.GetHostNetworkSpecOpts()
	}

	if err := containers.CreateContainer(containerName, createOpts, extraSpecOpts); err != nil {
		slog.Error("ContainerDriver.RunDeployment", "msg", "create container failed", "error", err)
		return repository.UpdateStatus(deployment.ID, common.DeploymentStatus{StatusCode: common.DeploymentStatusError, Reason: err.Error()})
	}

	stdoutPath := path.Join(string(config.Get().Daemon.LogsPath), "containers", fmt.Sprintf("%s_stdout.log", containerName))
	stderrPath := path.Join(string(config.Get().Daemon.LogsPath), "containers", fmt.Sprintf("%s_stderr.log", containerName))
	task, err := containers.StartContainer(containerName, stdoutPath, stderrPath)
	if err != nil {
		slog.Error("ContainerDriver.RunDeployment", "msg", "update container state failed. rolling back...", "error", err)

		// Delete container if it failed
		err := containers.DeleteContainer(containerName, syscall.SIGKILL, false)
		if err != nil {
			slog.Error("ContainerDriver.RunDeployment", "msg", "delete container on rolling back failed", "error", err)
		}
		return err
	}

	var gwAddressPtr *net.IP
	var ipAddressPtr *net.IP

	// Attach network if configured (only for bridge mode, not host mode)
	if c.Config.Network != nil && c.Config.Network.Mode != "host" {
		slog.Info("ContainerDriver.RunDeployment", "msg", "attaching network", "container", containerName, "network", c.Config.Network.Network)

		if err, _, gwAddress, ipAddress := network.StartNetwork(containerName, *c.Config.Network); err != nil {
			slog.Error("ContainerDriver.RunDeployment", "msg", "failed to attach network. rolling back...", "error", err)

			// Kill the task
			if task != nil {
				ctx, client, _ := cruntime.CreateClient()
				if client != nil {
					defer client.Close()
					task.Kill(ctx, syscall.SIGKILL)
					task.Wait(ctx)
					task.Delete(ctx)
				}
			} else {
				gwAddressPtr = &gwAddress
				ipAddressPtr = &ipAddress
			}

			if err := containers.DeleteContainer(containerName, syscall.SIGKILL, true); err != nil {
				slog.Error("ContainerDriver.RunDeployment", "msg", "delete container on network rollback failed", "error", err)
			}
			return fmt.Errorf("failed to attach network: %w", err)
		}

		slog.Info("ContainerDriver.RunDeployment", "msg", "network attached successfully", "container", containerName)
	}

	if err := common.UpdateMetadata(repository, deployment.ID, func(metadata *ContainerEntryMetadata) error {
		metadata.ContainerName = containerName
		metadata.GWAddress = gwAddressPtr
		metadata.IPAddress = ipAddressPtr
		metadata.StdoutPath = stdoutPath
		metadata.StderrPath = stderrPath
		return nil
	}); err != nil {
		return err
	}

	if err := repository.UpdateStatus(deployment.ID, common.DeploymentStatus{StatusCode: common.DeploymentStatusRunning}); err != nil {
		slog.Error("ContainerDriver.RunDeployment", "msg", "failed to update deployment status", "error", err)
		return err
	}

	slog.Info("ContainerDriver.RunDeployment", "msg", "container started", "container", containerName, "taskPID", task.Pid())

	return nil
}

func (c ContainerDriver) StopDeployment(repository common.DeploymentsRepository, deployment common.Deployment) error {
	// Verify deployment is in "running" state
	if deployment.Status.StatusCode != common.DeploymentStatusRunning {
		return fmt.Errorf("deployment %s is not in running state", deployment.ID)
	}

	var metadata ContainerEntryMetadata
	if tmp, err := json.Marshal(deployment.Metadata); err != nil {
		slog.Error("ContainerDriver.StopDeployment", "error", "error on retrieving metadata", "deployment", deployment.ID)
		return repository.UpdateStatus(deployment.ID, common.DeploymentStatus{StatusCode: common.DeploymentStatusError, Reason: err.Error()})
	} else {
		json.Unmarshal(tmp, &metadata)
	}

	if len(metadata.ContainerName) > 0 {
		if err := c.cleanupContainer(metadata.ContainerName, syscall.SIGTERM, false); err != nil {
			slog.Error("ContainerDriver.StopDeployment", "msg", "failed to clean-up container", "deploymentID", deployment.ID, "container", metadata.ContainerName, "error", err)
			return repository.UpdateStatus(deployment.ID, common.DeploymentStatus{StatusCode: common.DeploymentStatusError, Reason: err.Error()})
		} else {
			slog.Info("ContainerDriver.StopDeployment", "msg", "container cleaned up", "container", metadata.ContainerName)
		}
	}

	if err := common.UpdateMetadata(repository, deployment.ID, func(metadata *ContainerEntryMetadata) error {
		metadata.ContainerName = ""
		return nil
	}); err != nil {
		slog.Error("ContainerDriver.StopDeployment", "msg", "failed updating metadata", "deploymentID", deployment.ID, "error", err)
		return repository.UpdateStatus(deployment.ID, common.DeploymentStatus{StatusCode: common.DeploymentStatusError, Reason: err.Error()})
	}

	if err := repository.UpdateStatus(deployment.ID, common.DeploymentStatus{StatusCode: common.DeploymentStatusStopped}); err != nil {
		slog.Error("ContainerDriver.StopDeployment", "msg", "failed on status update", "deploymentID", deployment.ID, "error", err)
		return repository.UpdateStatus(deployment.ID, common.DeploymentStatus{StatusCode: common.DeploymentStatusError, Reason: err.Error()})
	}

	slog.Info("ContainerDriver.StopDeployment", "msg", "stop deployment", "deployment", deployment.ID)
	return nil
}

func (c ContainerDriver) cleanupContainer(containerName string, signal syscall.Signal, shallRemoveVolume bool) error {
	// Detach network before killing the task (network needs the netns which requires the process to be alive)
	// Only detach if not using host mode (host mode doesn't use CNI networking)
	if c.Config.Network != nil && c.Config.Network.Mode != "host" {
		slog.Info("ContainerDriver.cleanupContainer", "msg", "detaching network", "container", containerName)

		if err := network.DeleteNetwork(containerName); err != nil {
			slog.Warn("ContainerDriver.cleanupContainer", "msg", "failed to detach network", "container", containerName, "error", err)
			// Continue with container cleanup even if network cleanup fails
		} else {
			slog.Info("ContainerDriver.cleanupContainer", "msg", "network detached successfully", "container", containerName)
		}
	}
	return containers.DeleteContainer(containerName, signal, shallRemoveVolume)
}

func (c ContainerDriver) UpdateDeploymentStatus(r common.DeploymentsRepository, d common.Deployment) (common.DeploymentStatus, error) {
	var metadata ContainerEntryMetadata
	if tmp, err := json.Marshal(d.Metadata); err != nil {
		slog.Error("ContainerDriver.GetDeploymentStatus", "error", "error on retrieving metadata", "deployment", d.ID)
		return common.DeploymentStatus{StatusCode: common.DeploymentStatusError, Reason: err.Error()}, nil
	} else {
		json.Unmarshal(tmp, &metadata)
	}

	status, err := api.GetContainerStatus(metadata.ContainerName)
	if err != nil {
		return common.DeploymentStatus{StatusCode: common.DeploymentStatusError, Reason: err.Error()}, nil
	}

	s := common.DeploymentStatus{}

	switch status.Status {
	case containerd.Pausing:
	case containerd.Running:
		s = common.DeploymentStatus{StatusCode: common.DeploymentStatusRunning}
	case containerd.Stopped:
		s = common.DeploymentStatus{StatusCode: common.DeploymentStatusStopped}
	case containerd.Paused:
	case containerd.Created:
		s = common.DeploymentStatus{StatusCode: common.DeploymentStatusPlanned}
	}

	if err := r.UpdateStatus(d.ID, s); err != nil {
		slog.Error("ContainerDriver.UpdateDeploymentStatus", "msg", "failed to update deployment status", "error", err)
		return common.DeploymentStatus{}, err
	}

	return common.DeploymentStatus{StatusCode: common.DeploymentStatusError, Reason: fmt.Sprintf("container state is %v", status.Status)}, nil
}

func (c ContainerDriver) GetDriverConfig() common.LoadDriverConfig {
	return common.LoadDriverConfig{Driver: ContainerDriverID, DriverConfig: c.Config}
}

func (c ContainerDriver) StreamStdout(repository common.DeploymentsRepository, deployment common.Deployment, w io.Writer) error {
	var metadata ContainerEntryMetadata
	if tmp, err := json.Marshal(deployment.Metadata); err != nil {
		slog.Error("ContainerDriver.StopDeployment", "error", "error on retrieving metadata", "deployment", deployment.ID)
		return err
	} else {
		json.Unmarshal(tmp, &metadata)
	}

	if len(metadata.StdoutPath) == 0 {
		return fmt.Errorf("stdout path empty")
	}

	return common.TailFile(metadata.StdoutPath, w)
}

func (c ContainerDriver) StreamStderr(repository common.DeploymentsRepository, deployment common.Deployment, w io.Writer) error {
	var metadata ContainerEntryMetadata
	if tmp, err := json.Marshal(deployment.Metadata); err != nil {
		slog.Error("ContainerDriver.ReadStderr", "error", "error on retrieving metadata", "deployment", deployment.ID)
		return err
	} else {
		json.Unmarshal(tmp, &metadata)
	}

	if len(metadata.StderrPath) == 0 {
		return fmt.Errorf("stderr path empty")
	}

	return common.TailFile(metadata.StderrPath, w)
}

func (c ContainerDriver) ReadStdout(repository common.DeploymentsRepository, deployment common.Deployment, offset int64) ([]byte, int64, error) {
	var metadata ContainerEntryMetadata
	if tmp, err := json.Marshal(deployment.Metadata); err != nil {
		slog.Error("ContainerDriver.ReadStdout", "error", "error on retrieving metadata", "deployment", deployment.ID)
		return nil, 0, err
	} else {
		json.Unmarshal(tmp, &metadata)
	}

	if len(metadata.StdoutPath) == 0 {
		return nil, 0, fmt.Errorf("stdout path empty")
	}

	return common.ReadFileAt(metadata.StdoutPath, offset)
}

func (c ContainerDriver) ReadStderr(repository common.DeploymentsRepository, deployment common.Deployment, offset int64) ([]byte, int64, error) {
	var metadata ContainerEntryMetadata
	if tmp, err := json.Marshal(deployment.Metadata); err != nil {
		slog.Error("ContainerDriver.ReadStderr", "error", "error on retrieving metadata", "deployment", deployment.ID)
		return nil, 0, err
	} else {
		json.Unmarshal(tmp, &metadata)
	}

	if len(metadata.StderrPath) == 0 {
		return nil, 0, fmt.Errorf("stderr path empty")
	}

	return common.ReadFileAt(metadata.StderrPath, offset)
}
