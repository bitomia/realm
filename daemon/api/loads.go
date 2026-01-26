package api

import (
	"fmt"
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

	// Group deployments by load name
	loadDeployments := make(map[string][]common.Deployment)
	for _, d := range allDeployments {
		loadDeployments[d.LoadName] = append(loadDeployments[d.LoadName], d)
	}

	var response dto.LoadsDeployments
	for loadName, deployments := range loadDeployments {
		var state common.DeploymentState
		for _, d := range deployments {
			switch d.State {
			case common.DeploymentStateRunning:
				state = common.DeploymentStateRunning
			case common.DeploymentStatePlanned:
				state = common.DeploymentStatePlanned
			}

			response = append(response, dto.LoadDeployment{
				LoadName:     loadName,
				DeploymentId: d.ID.String(),
				State:        state,
				Driver:       string(d.LoadDriver.GetLoadDriverID()),
				DriverConfig: d.LoadDriver.GetDriverConfig().DriverConfig,
			})
		}
	}

	return &response, nil
}

func StartLoadDeployments(loadName string) error {
	database := db.GetDB()

	// Get planned deployments for this load
	deployments, err := database.DeploymentsRepository.GetByLoadAndState(loadName, common.DeploymentStatePlanned)
	if err != nil {
		return err
	}

	if len(deployments) == 0 {
		return fmt.Errorf("No planned deployment found. Run 'plan' first.")
	}

	// Start all planned deployments
	for _, deployment := range deployments {
		slog.Info("loads.StartLoadHandler", "load", loadName, "deployment", deployment.ID, "msg", "starting deployment")
		if err := deployment.LoadDriver.StartDeployment(database.DeploymentsRepository, deployment); err != nil {
			return err
		}
		slog.Info("loads.StartLoadHandler", "msg", "load deployment started", "deploymentID", deployment.ID)
	}

	return nil
}

func StopLoadDeployments(loadName string) error {
	database := db.GetDB()

	// Only get RUNNING deployments (not planned ones)
	deployments, err := database.DeploymentsRepository.GetByLoadAndState(loadName, common.DeploymentStateRunning)
	if err != nil {
		return err
	}
	if len(deployments) == 0 {
		return fmt.Errorf("No running deployments found")
	}

	for _, deployment := range deployments {
		slog.Info("loads.StopLoadDeploymentsHandler", "loadName", loadName, "driverID", deployment.LoadDriver.GetLoadDriverID())
		if err := deployment.LoadDriver.StopDeployment(database.DeploymentsRepository, deployment); err != nil {
			return err
		}
		slog.Info("loads.StopLoadDeploymentsHandler", "msg", "load deployments stopped", "deploymentID", deployment.ID)
	}
	return nil
}

func PlanAndRegisterLoad(load *common.Load) (*dto.PlanLoadInfo, error) {
	database := db.GetDB()

	deploymentID, err := load.Driver.PlanAndRegister(database.DeploymentsRepository, load.Name)
	if err != nil {
		return nil, err
	}

	return &dto.PlanLoadInfo{DeploymentId: deploymentID.String()}, nil
}

func UnplanLoad(loadName string) error {
	database := db.GetDB()

	// Only get PLANNED deployments
	deployments, err := database.DeploymentsRepository.GetByLoadAndState(loadName, common.DeploymentStatePlanned)
	if err != nil {
		return err
	}

	if len(deployments) == 0 {
		return fmt.Errorf("No planned deployments found")
	}

	for _, deployment := range deployments {
		slog.Info("loads.UnplanLoadHandler", "loadName", loadName, "driverID", deployment.LoadDriver.GetLoadDriverID())

		if err := deployment.LoadDriver.UnplanDeployment(database.DeploymentsRepository, deployment); err != nil {
			return err
		}
	}

	return nil
}
