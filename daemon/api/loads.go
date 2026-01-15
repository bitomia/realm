package api

import (
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
			})
		}
	}

	return &response, nil
}
