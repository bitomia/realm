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

	if err := CheckDeploymentStatus(deployments, &database.DeploymentsRepository, func(s common.DeploymentStatusCode) bool {
		return s == common.DeploymentStatusPlanned || s == common.DeploymentStatusStopped
	}, "RunLoadDeployments"); err != nil {
		return err
	}

	// Start all planned deployments
	for _, deployment := range deployments {
		slog.Info("RunLoadDeployments", "load", loadName, "deployment", deployment.ID, "msg", "starting deployment")
		if err := deployment.LoadDriver.RunDeployment(database.DeploymentsRepository, deployment); err != nil {
			return err
		}
		slog.Info("RunLoadDeployments", "msg", "deployment started", "deploymentID", deployment.ID)
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

	if err := CheckDeploymentStatus(deployments, &database.DeploymentsRepository, func(s common.DeploymentStatusCode) bool {
		return s == common.DeploymentStatusRunning
	}, "StopLoadDeployments"); err != nil {
		return err
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

func KillLoadDeployments(loadName string) error {
	database := db.GetDB()

	// Get deployments for this load
	deployments, err := database.DeploymentsRepository.GetByLoad(loadName)
	if err != nil {
		return err
	}

	if len(deployments) == 0 {
		return fmt.Errorf("No running deployments found")
	}

	if err := CheckDeploymentStatus(deployments, &database.DeploymentsRepository, func(s common.DeploymentStatusCode) bool {
		return s == common.DeploymentStatusRunning
	}, "KillLoadDeployments"); err != nil {
		return err
	}

	// Kill deployments
	for _, deployment := range deployments {
		slog.Info("KillLoadDeployments", "loadName", loadName, "driverID", deployment.LoadDriver.GetLoadDriverID())
		if err := deployment.LoadDriver.KillDeployment(database.DeploymentsRepository, deployment); err != nil {
			return err
		}
		slog.Info("KillLoadDeployments", "msg", "load deployments killed", "deploymentID", deployment.ID)
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

func UnplanLoadDeployments(loadName string) error {
	database := db.GetDB()

	deployments, err := database.DeploymentsRepository.GetByLoad(loadName)
	if err != nil {
		return err
	}

	if len(deployments) == 0 {
		return fmt.Errorf("No deployments found")
	}

	if err := CheckDeploymentStatus(deployments, &database.DeploymentsRepository, func(s common.DeploymentStatusCode) bool {
		return s == common.DeploymentStatusError || s == common.DeploymentStatusStopped || s == common.DeploymentStatusPlanned
	}, "UnplanLoadDeployments"); err != nil {
		return err
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

func CheckDeploymentStatus(deployments []common.Deployment, repository *common.DeploymentsRepository, check func(common.DeploymentStatusCode) bool, msgContext string) error {
	for _, d := range deployments {
		status, err := d.LoadDriver.UpdateDeploymentStatus(*repository, d)
		if err != nil {
			err := fmt.Errorf("Cannot update deployment status %s", d.ID)
			slog.Error(msgContext, "deployment", d.ID, "msg", err)
			return err
		}
		d.Status = status

		if !check(d.Status.StatusCode) {
			err := fmt.Errorf("Cannot run deployment %s: deployment is in %s state", d.ID, d.Status.StatusCode)
			slog.Error(msgContext, "deployment", d.ID, "msg", err)
			return err
		}
	}
	return nil
}
