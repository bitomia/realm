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
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/oci"
	"github.com/go-viper/mapstructure/v2"
	"github.com/google/uuid"

	"github.com/bitomia/realm/agent/api"
	"github.com/bitomia/realm/agent/capabilities"
	"github.com/bitomia/realm/agent/config"
	"github.com/bitomia/realm/agent/containers"
	"github.com/bitomia/realm/agent/cruntime"
	"github.com/bitomia/realm/agent/network"
	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/common/dto"
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

func NewContainerDriver(c any) (common.LoadDriver, error) {
	var config ContainerConfig
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		// Configure mapstructure decoder to use 'json' tags
		// because it has to work for config files (yaml)
		// and for remote commands (json)
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

	if err := driver.verifyConfig(); err != nil {
		return nil, err
	}
	return driver, nil
}

func (c *ContainerDriver) ID() common.LoadDriverID {
	return ContainerDriverID
}

func (c *ContainerDriver) Info() common.LoadDriverInfo {
	return common.LoadDriverInfo{
		ID:  ContainerDriverID,
		New: NewContainerDriver,
	}
}

func (c *ContainerDriver) verifyConfig() error {
	if c.Config.Image == "" {
		return fmt.Errorf("container image not specified")
	}

	// Validate network configuration
	if c.Config.Network != nil {
		mode := c.Config.Network.Mode
		if mode == "" {
			mode = "bridge" // default mode
		}

		// Validate mode is valid
		if mode != "bridge" && mode != "host" {
			return fmt.Errorf("invalid network mode '%s': must be 'bridge' or 'host'", mode)
		}

		// If not using host mode, network name is required
		if mode != "host" && c.Config.Network.Network == "" {
			return fmt.Errorf("network name is required when using bridge mode")
		}

		// Warn during provisioning if host mode has conflicting settings
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

func (c *ContainerDriver) Provision(nodeDriver common.NodeDriver, repository common.DeploymentsRepository, loadName string) (common.DeploymentID, error) {
	if nodeDriver == nil {
		err := fmt.Errorf("nil node driver")
		slog.Error("ContainerDriver.Provision", "error", err)
		return uuid.Nil, err
	}
	sysCaps := capabilities.Get()
	if !sysCaps.ContainersEngine() {
		err := fmt.Errorf("containers engine capability required")
		slog.Error("ContainerDriver.Provision", "error", err)
		return uuid.Nil, err
	}
	if !sysCaps.ContainersNetworking() {
		err := fmt.Errorf("containers networking capability required")
		slog.Error("ContainerDriver.Provision", "error", err)
		return uuid.Nil, err
	}
	if !sysCaps.Volumes() {
		err := fmt.Errorf("volumes capability required")
		slog.Error("ContainerDriver.Provision", "error", err)
		return uuid.Nil, err
	}

	// Try to pull and get image
	ctx, client, err := cruntime.CreateClient()
	if err != nil {
		slog.Error("ContainerDriver.Provision", "error", err)
		return uuid.Nil, err
	}
	defer client.Close()

	_, err = containers.TryPullAndGetImage(ctx, client, c.Config.Image)
	if err != nil {
		slog.Error("ContainerDriver.Provision", "error", err)
		return uuid.Nil, err
	}

	did, err := repository.Create(loadName, c, common.DeploymentStatus{StatusCode: common.DeploymentStatusReady}, ContainerEntryMetadata{})
	if err != nil {
		slog.Error("ContainerDriver.Provision", "msg", "failed to create deployment", "error", err)
		return uuid.Nil, err
	}

	return did, nil
}

func (c *ContainerDriver) Deprovision(repository common.DeploymentsRepository, deployment common.Deployment) error {
	slog.Info("ContainerDriver.Deprovision", "msg", "deprovisioning deployment", "deployment", deployment.ID)

	// Clean-up if error
	if deployment.Status.StatusCode == common.DeploymentStatusError {
		metadata, err := getContainerMetadata(deployment)
		if err != nil {
			slog.Warn("ContainerDriver.Deprovision", "error", "error on retrieving metadata", "deployment", deployment.ID)
			goto deprovision_deployment
		}

		if len(metadata.ContainerName) > 0 {
			if err := c.cleanupContainer(metadata.ContainerName, syscall.SIGKILL, false); err != nil {
				slog.Warn("ContainerDriver.Deprovision", "msg", "failed to clean-up container. possible orphan container", "deploymentID", deployment.ID, "container", metadata.ContainerName, "error", err)
			}
		}
	}

deprovision_deployment:
	if err := repository.DeleteDeployment(deployment.ID); err != nil {
		slog.Error("ContainerDriver.Deprovision", "msg", "failed to delete deployment", "deploymentID", deployment.ID, "error", err)

		return repository.UpdateStatus(deployment.ID, common.DeploymentStatus{StatusCode: common.DeploymentStatusError, Reason: fmt.Sprintf("failed to delete deployment: %s", err.Error())})
	}

	return nil
}

func (c *ContainerDriver) Start(repository common.DeploymentsRepository, deployment common.Deployment) error {
	// Use loadName to create a unique container name
	containerName := fmt.Sprintf("%s-%s", deployment.LoadName, uuid.New())
	slog.Info("ContainerDriver.Start", "msg", "starting container", "container", containerName)

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
		slog.Info("ContainerDriver.Start", "msg", "using host network mode", "container", containerName)
		extraSpecOpts = containers.GetHostNetworkSpecOpts()
	}

	if err := containers.CreateContainer(containerName, createOpts, extraSpecOpts); err != nil {
		slog.Error("ContainerDriver.Start", "msg", "create container failed", "error", err)
		return repository.UpdateStatus(deployment.ID, common.DeploymentStatus{StatusCode: common.DeploymentStatusError, Reason: err.Error()})
	}

	stdoutPath := path.Join(config.Get().DataPath, "logs", "containers", fmt.Sprintf("%s_stdout.log", containerName))
	stderrPath := path.Join(config.Get().DataPath, "logs", "containers", fmt.Sprintf("%s_stderr.log", containerName))
	task, err := containers.StartContainer(containerName, stdoutPath, stderrPath)
	if err != nil {
		err = fmt.Errorf("update container state failed. rolling back: %s", err.Error())
		slog.Error("ContainerDriver.Start", "msg", "rolling back update container", "error", err)

		// Delete container if it failed
		if err := containers.DeleteContainer(containerName, syscall.SIGKILL, false); err != nil {
			slog.Error("ContainerDriver.Start", "msg", "delete container on rolling back failed", "error", err)
		}

		return err
	}

	var gwAddressPtr *net.IP
	var ipAddressPtr *net.IP

	// Attach network if configured (only for bridge mode, not host mode)
	if c.Config.Network != nil && c.Config.Network.Mode != "host" {
		slog.Info("ContainerDriver.Start", "msg", "attaching network", "container", containerName, "network", c.Config.Network.Network)

		if _, gwAddress, ipAddress, err := network.StartNetwork(containerName, *c.Config.Network); err != nil {
			err = fmt.Errorf("failed to attach network. rolling back: %s", err.Error())
			slog.Error("ContainerDriver.Start", "msg", "failed to attach network", "error", err)

			// Kill the task
			if task != nil {
				ctx, client, _ := cruntime.CreateClient()
				if client != nil {
					defer client.Close()
					_ = task.Kill(ctx, syscall.SIGKILL)
					_, _ = task.Wait(ctx)
					_, _ = task.Delete(ctx)
				}
			} else {
				gwAddressPtr = &gwAddress
				ipAddressPtr = &ipAddress
			}

			if err := containers.DeleteContainer(containerName, syscall.SIGKILL, true); err != nil {
				slog.Error("ContainerDriver.Start", "msg", "delete container on network rollback failed", "error", err)
			}

			return err
		}

		slog.Info("ContainerDriver.Start", "msg", "network attached successfully", "container", containerName)
	}

	if err := common.UpdateDeploymentMetadata(repository, deployment.ID, func(metadata *ContainerEntryMetadata) error {
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
		slog.Error("ContainerDriver.Start", "msg", "failed to update deployment status", "error", err)
		return err
	}

	slog.Info("ContainerDriver.Start", "msg", "container started", "container", containerName, "taskPID", task.Pid())
	return nil
}

func (c *ContainerDriver) Stop(repository common.DeploymentsRepository, deployment common.Deployment) error {
	metadata, err := getContainerMetadata(deployment)
	if err != nil {
		return repository.UpdateStatus(deployment.ID, common.DeploymentStatus{StatusCode: common.DeploymentStatusError, Reason: err.Error()})
	}

	if len(metadata.ContainerName) > 0 {
		if err := c.cleanupContainer(metadata.ContainerName, syscall.SIGTERM, false); err != nil {
			slog.Error("ContainerDriver.Stop", "msg", "failed to clean-up container", "deploymentID", deployment.ID, "container", metadata.ContainerName, "error", err)
			return repository.UpdateStatus(deployment.ID, common.DeploymentStatus{StatusCode: common.DeploymentStatusError, Reason: err.Error()})
		} else {
			slog.Info("ContainerDriver.Stop", "msg", "container cleaned up", "container", metadata.ContainerName)
		}
	}

	if err := common.UpdateDeploymentMetadata(repository, deployment.ID, func(metadata *ContainerEntryMetadata) error {
		metadata.ContainerName = ""
		return nil
	}); err != nil {
		slog.Error("ContainerDriver.Stop", "msg", "failed updating metadata", "deploymentID", deployment.ID, "error", err)
		return repository.UpdateStatus(deployment.ID, common.DeploymentStatus{StatusCode: common.DeploymentStatusError, Reason: err.Error()})
	}

	if err := repository.UpdateStatus(deployment.ID, common.DeploymentStatus{StatusCode: common.DeploymentStatusStopped}); err != nil {
		slog.Error("ContainerDriver.Stop", "msg", "failed on status update", "deploymentID", deployment.ID, "error", err)
		return repository.UpdateStatus(deployment.ID, common.DeploymentStatus{StatusCode: common.DeploymentStatusError, Reason: err.Error()})
	}

	slog.Info("ContainerDriver.Stop", "msg", "stop deployment", "deployment", deployment.ID)
	return nil
}

func (c *ContainerDriver) Kill(repository common.DeploymentsRepository, deployment common.Deployment) error {
	metadata, err := getContainerMetadata(deployment)
	if err != nil {
		return repository.UpdateStatus(deployment.ID, common.DeploymentStatus{StatusCode: common.DeploymentStatusError, Reason: err.Error()})
	}

	if len(metadata.ContainerName) > 0 {
		if err := c.cleanupContainer(metadata.ContainerName, syscall.SIGKILL, false); err != nil {
			slog.Error("ContainerDriver.Kill", "msg", "failed to clean-up container", "deploymentID", deployment.ID, "container", metadata.ContainerName, "error", err)
			return repository.UpdateStatus(deployment.ID, common.DeploymentStatus{StatusCode: common.DeploymentStatusError, Reason: err.Error()})
		} else {
			slog.Info("ContainerDriver.Kill", "msg", "container cleaned up", "container", metadata.ContainerName)
		}
	}

	if err := common.UpdateDeploymentMetadata(repository, deployment.ID, func(metadata *ContainerEntryMetadata) error {
		metadata.ContainerName = ""
		return nil
	}); err != nil {
		slog.Error("ContainerDriver.Kill", "msg", "failed updating metadata", "deploymentID", deployment.ID, "error", err)
		return repository.UpdateStatus(deployment.ID, common.DeploymentStatus{StatusCode: common.DeploymentStatusError, Reason: err.Error()})
	}

	if err := repository.UpdateStatus(deployment.ID, common.DeploymentStatus{StatusCode: common.DeploymentStatusStopped}); err != nil {
		slog.Error("ContainerDriver.Kill", "msg", "failed on status update", "deploymentID", deployment.ID, "error", err)
		return repository.UpdateStatus(deployment.ID, common.DeploymentStatus{StatusCode: common.DeploymentStatusError, Reason: err.Error()})
	}

	slog.Info("ContainerDriver.Kill", "msg", "kill deployment", "deployment", deployment.ID)
	return nil
}

func (c *ContainerDriver) cleanupContainer(containerName string, signal syscall.Signal, shallRemoveVolume bool) error {
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

func (c *ContainerDriver) UpdateStatus(r common.DeploymentsRepository, d common.Deployment) (common.DeploymentStatus, error) {
	metadata, err := getContainerMetadata(d)
	if err != nil {
		return common.DeploymentStatus{StatusCode: common.DeploymentStatusError, Reason: err.Error()}, nil
	}

	status, err := api.GetContainerStatus(metadata.ContainerName)
	if err != nil {
		if errdefs.IsNotFound(err) {
			if d.Status.StatusCode == common.DeploymentStatusReady || d.Status.StatusCode == common.DeploymentStatusStopped {
				return d.Status, nil
			} else {
				slog.Warn("ContainerDriver.UpdateStatus", "msg", "not found container status", "deployment", d.ID, "err", err)
				return common.DeploymentStatus{StatusCode: common.DeploymentStatusError, Reason: err.Error()}, nil
			}
		} else {
			slog.Warn("ContainerDriver.UpdateStatus", "msg", "getting container status failed ", "deployment", d.ID, "err", err)
		}
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
		s = common.DeploymentStatus{StatusCode: common.DeploymentStatusReady}
	}

	if err := r.UpdateStatus(d.ID, s); err != nil {
		slog.Error("ContainerDriver.UpdateStatus", "msg", "failed to update deployment status", "error", err)
		return common.DeploymentStatus{}, err
	}

	return s, nil
}

func (c *ContainerDriver) GetDriverConfig() common.LoadDriverConfig {
	return common.LoadDriverConfig{Driver: ContainerDriverID, DriverConfig: c.Config}
}

func (c *ContainerDriver) StreamStdout(repository common.DeploymentsRepository, deployment common.Deployment, w io.Writer) error {
	metadata, err := getContainerMetadata(deployment)
	if err != nil {
		return err
	}

	if len(metadata.StdoutPath) == 0 {
		return fmt.Errorf("stdout path empty")
	}

	return common.TailFile(metadata.StdoutPath, w)
}

func (c *ContainerDriver) StreamStderr(repository common.DeploymentsRepository, deployment common.Deployment, w io.Writer) error {
	metadata, err := getContainerMetadata(deployment)
	if err != nil {
		return err
	}

	if len(metadata.StderrPath) == 0 {
		return fmt.Errorf("stderr path empty")
	}

	return common.TailFile(metadata.StderrPath, w)
}

func (c *ContainerDriver) ReadStdout(repository common.DeploymentsRepository, deployment common.Deployment, offset int64) ([]byte, int64, error) {
	metadata, err := getContainerMetadata(deployment)
	if err != nil {
		return nil, 0, err
	}

	if len(metadata.StdoutPath) == 0 {
		return nil, 0, fmt.Errorf("stdout path empty")
	}

	return common.ReadFileAt(metadata.StdoutPath, offset)
}

func (c *ContainerDriver) ReadStderr(repository common.DeploymentsRepository, deployment common.Deployment, offset int64) ([]byte, int64, error) {
	metadata, err := getContainerMetadata(deployment)
	if err != nil {
		return nil, 0, err
	}

	if len(metadata.StderrPath) == 0 {
		return nil, 0, fmt.Errorf("stderr path empty")
	}

	return common.ReadFileAt(metadata.StderrPath, offset)
}

func getContainerMetadata(d common.Deployment) (*ContainerEntryMetadata, error) {
	var metadata ContainerEntryMetadata
	if tmp, err := json.Marshal(d.Metadata); err != nil {
		slog.Error("ProcessDriver.getMetadata", "error", "error on marshaling metadata", "deployment", d.ID)
		return nil, err
	} else {
		if err := json.Unmarshal(tmp, &metadata); err != nil {
			slog.Error("ProcessDriver.getMetadata", "error", "error on unmarshalling metadata", "deployment", d.ID)
			return nil, err
		}
	}
	return &metadata, nil
}
