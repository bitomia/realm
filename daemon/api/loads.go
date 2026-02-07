package api

import (
	"fmt"
	"io"
	"log/slog"

	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/common/dto"
	"github.com/bitomia/realm/daemon/db"
)

func GetLoadsDeployments() (*dto.LoadsDeployments, error) {
	database := db.GetDB()
	allDeployments, err := database.DeploymentsRepository.GetAll()
	if err != nil {
		return nil, err
	}

	var response dto.LoadsDeployments
	for _, d := range allDeployments {
		status, err := d.LoadDriver.UpdateDeploymentStatus(database.DeploymentsRepository, d)
		if err != nil {
			slog.Error("GetLoadsDeployments", "error", err)
			return nil, err
		}
		response = append(response, dto.LoadDeployment{
			LoadName:         d.LoadName,
			DeploymentId:     d.ID.String(),
			DeploymentStatus: status,
			Driver:           string(d.LoadDriver.GetLoadDriverID()),
			DriverConfig:     d.LoadDriver.GetDriverConfig().DriverConfig,
			Metadata:         d.Metadata,
		})
	}

	return &response, nil
}

func RunLoadDeployments(loadName string) error {
	database := db.GetDB()

	// Get deployments for this load
	deployments, err := database.DeploymentsRepository.GetByLoad(loadName)
	if err != nil {
		return err
	}

	if len(deployments) == 0 {
		return fmt.Errorf("No planned deployment found. Run 'plan' first.")
	}

	// Verify deployments are in planned state
	for _, deployment := range deployments {
		if deployment.Status.StatusCode != common.DeploymentStatusPlanned && deployment.Status.StatusCode != common.DeploymentStatusStopped {
			err := fmt.Errorf("Cannot run deployment %s: deployments must be in planned or stopped state, but deployment is in %s state", deployment.ID, deployment.Status.StatusCode)
			slog.Error("RunLoadDeployments", "deployment", deployment.ID, "msg", err)

			return err
		}
	}

	// Start all planned deployments
	for _, deployment := range deployments {
		slog.Info("RunLoadDeployments", "load", loadName, "deployment", deployment.ID, "msg", "starting deployment")
		if err := deployment.LoadDriver.RunDeployment(database.DeploymentsRepository, deployment); err != nil {
			return err
		}
		slog.Info("RunLoadDeployments", "msg", "load deployment started", "deploymentID", deployment.ID)
	}

	return nil
}

func StopLoadDeployments(loadName string) error {
	database := db.GetDB()

	// Get deployments for this load
	deployments, err := database.DeploymentsRepository.GetByLoad(loadName)
	if err != nil {
		return err
	}

	if len(deployments) == 0 {
		return fmt.Errorf("No running deployments found")
	}

	// Verify deployments are in running state
	for _, deployment := range deployments {
		if deployment.Status.StatusCode != common.DeploymentStatusRunning {
			err := fmt.Errorf("Cannot stop deployment %s: deployments must be in running state, but deployment is in %s state", deployment.ID, deployment.Status.StatusCode)
			slog.Error("StopLoadDeployments", "deployment", deployment.ID, "msg", err)

			return err
		}
	}

	// Stop deployments
	for _, deployment := range deployments {
		slog.Info("StopLoadDeployments", "loadName", loadName, "driverID", deployment.LoadDriver.GetLoadDriverID())
		if err := deployment.LoadDriver.StopDeployment(database.DeploymentsRepository, deployment); err != nil {
			return err
		}
		slog.Info("StopLoadDeployments", "msg", "load deployments stopped", "deploymentID", deployment.ID)
	}
	return nil
}

func PlanLoad(load *common.Load) (*dto.PlanLoadInfo, error) {
	database := db.GetDB()

	// Check if deployments already exist for this load
	existingDeployments, err := database.DeploymentsRepository.GetByLoad(load.Name)
	if err != nil {
		return nil, err
	}

	if len(existingDeployments) > 0 {
		err := fmt.Errorf("Cannot plan load: deployment already exists for load '%s'. Run 'unplan' first.", load.Name)
		slog.Error("PlanLoad", "error", err)

		return nil, err
	}

	deploymentID, err := load.Driver.PlanDeployment(database.DeploymentsRepository, load.Name)
	if err != nil {
		return nil, err
	}

	return &dto.PlanLoadInfo{DeploymentId: deploymentID.String()}, nil
}

func UnplanLoad(loadName string) error {
	database := db.GetDB()

	deployments, err := database.DeploymentsRepository.GetByLoad(loadName)
	if err != nil {
		return err
	}

	if len(deployments) == 0 {
		return fmt.Errorf("No deployments found")
	}

	// Verify deployments are in error or stopped state
	for _, deployment := range deployments {
		if deployment.Status.StatusCode != common.DeploymentStatusError && deployment.Status.StatusCode != common.DeploymentStatusStopped && deployment.Status.StatusCode != common.DeploymentStatusPlanned {
			err := fmt.Errorf("Cannot unplan deployment %s: deployment must be in planned, stopped or error state. Current state is %s", deployment.ID, deployment.Status.StatusCode)
			slog.Error("UnplanLoad", "error", err)

			return err
		}
	}

	for _, deployment := range deployments {
		slog.Info("UnplanLoad", "loadName", loadName, "driverID", deployment.LoadDriver.GetLoadDriverID())
		if err := deployment.LoadDriver.UnplanDeployment(database.DeploymentsRepository, deployment); err != nil {
			return err
		}
	}

	return nil
}

func StreamLoadStdout(loadName string, w io.Writer) error {
	database := db.GetDB()

	deployments, err := database.DeploymentsRepository.GetByLoad(loadName)
	if err != nil {
		return err
	}

	if len(deployments) == 0 {
		return fmt.Errorf("No planned deployments found")
	}

	if len(deployments) > 1 {
		return fmt.Errorf("More than one deployment found for this load: %s", loadName)
	}

	return deployments[0].LoadDriver.StreamStdout(database.DeploymentsRepository, deployments[0], w)
}

func StreamLoadStderr(loadName string, w io.Writer) error {
	database := db.GetDB()

	deployments, err := database.DeploymentsRepository.GetByLoad(loadName)
	if err != nil {
		return err
	}

	if len(deployments) == 0 {
		return fmt.Errorf("No planned deployments found")
	}

	if len(deployments) > 1 {
		return fmt.Errorf("More than one deployment found for this load: %s", loadName)
	}

	return deployments[0].LoadDriver.StreamStderr(database.DeploymentsRepository, deployments[0], w)
}

func ReadLoadStdout(loadName string, offset int64) ([]byte, int64, error) {
	database := db.GetDB()

	deployments, err := database.DeploymentsRepository.GetByLoad(loadName)
	if err != nil {
		return nil, 0, err
	}

	if len(deployments) == 0 {
		return nil, 0, fmt.Errorf("No planned deployments found")
	}

	if len(deployments) > 1 {
		return nil, 0, fmt.Errorf("More than one deployment found for this load: %s", loadName)
	}

	return deployments[0].LoadDriver.ReadStdout(database.DeploymentsRepository, deployments[0], offset)
}

func ReadLoadStderr(loadName string, offset int64) ([]byte, int64, error) {
	database := db.GetDB()

	deployments, err := database.DeploymentsRepository.GetByLoad(loadName)
	if err != nil {
		return nil, 0, err
	}

	if len(deployments) == 0 {
		return nil, 0, fmt.Errorf("No planned deployments found")
	}

	if len(deployments) > 1 {
		return nil, 0, fmt.Errorf("More than one deployment found for this load: %s", loadName)
	}

	return deployments[0].LoadDriver.ReadStderr(database.DeploymentsRepository, deployments[0], offset)
}
