package db

import (
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"

	"github.com/bitomia/realm/common"
)

type BoltDeploymentsRepository struct {
	db *AgentDB
}

type DeploymentValue struct {
	ID               common.DeploymentID     `json:"id"`
	LoadName         string                  `json:"load_name"`
	LoadDriverConfig common.LoadDriverConfig `json:"load_driver_config"`
	Status           common.DeploymentStatus `json:"status"`
	Metadata         any                     `json:"metadata"`
}

func (r *BoltDeploymentsRepository) Create(loadName string, driver common.LoadDriver, status common.DeploymentStatus, metadata any) (common.DeploymentID, error) {
	deployment := DeploymentValue{
		ID:               uuid.New(),
		LoadName:         loadName,
		Status:           status,
		LoadDriverConfig: driver.GetDriverConfig(),
		Metadata:         metadata,
	}

	slog.Info("BoltDeploymentsRepository.Create", "deploymentID", deployment.ID, "loadName", loadName)

	deploymentJson, err := json.Marshal(deployment)
	if err != nil {
		slog.Error("BoltDeploymentsRepository.Create", "deploymentID", deployment.ID, "msg", "Marshaling driver", "error", err.Error())
		return uuid.Nil, err
	}

	loadKey, err := r.db.loadDeploymentKey(loadName, deployment.ID)
	if err != nil {
		slog.Error("BoltDeploymentsRepository.Create", "deploymentID", deployment.ID, "msg", "creating load-deployment key", "error", err.Error())
		return uuid.Nil, err
	}
	deploymentKey, err := r.db.deploymentKey(deployment.ID)
	if err != nil {
		slog.Error("BoltDeploymentsRepository.Create", "deploymentID", deployment.ID, "msg", "creating deployment key", "error", err.Error())
		return uuid.Nil, err
	}

	kvs := map[string]string{
		deploymentKey: string(deploymentJson),
		loadKey:       deployment.ID.String(),
	}

	if err := r.db.putMulti(kvs); err != nil {
		slog.Error("BoltDeploymentsRepository.Create", "deploymentID", deployment.ID, "loadName", loadName, "msg", "db put", "error", err.Error())
		return uuid.Nil, err
	}

	return deployment.ID, nil
}

func (r *BoltDeploymentsRepository) GetByLoad(loadName string) ([]common.Deployment, error) {
	loadKey, err := r.db.loadKey(loadName)
	if err != nil {
		slog.Error("BoltDeploymentsRepository.GetByLoad", "load", loadName, "msg", "loadKey failed", "error", err.Error())
		return nil, err
	}

	loadDeployments, err := r.db.getPrefix(loadKey)
	if err != nil {
		slog.Error("BoltDeploymentsRepository.GetByLoad", "loadName", loadName, "msg", "retrieving load deployments", "error", err.Error())
		return nil, err
	}

	var deployments []common.Deployment
	for _, deploymentIDStr := range loadDeployments {
		deploymentID, err := uuid.Parse(deploymentIDStr)
		if err != nil {
			slog.Error("BoltDeploymentsRepository.GetByLoad", "loadName", loadName, "msg", "parsing deployment ID", "error", err.Error())
			return nil, err
		}

		deployment, err := r.getDeploymentValue(deploymentID)
		if err != nil {
			slog.Error("BoltDeploymentsRepository.GetByLoad", "deploymentID", deploymentID, "msg", "getting deployment", "error", err.Error())
			return nil, err
		}

		if loadDriver, err := common.BuildLoadDriver(deployment.LoadDriverConfig); err != nil {
			return nil, err
		} else {
			deployments = append(deployments, common.Deployment{
				ID:         deployment.ID,
				LoadName:   loadName,
				LoadDriver: loadDriver,
				Status:     deployment.Status,
				Metadata:   deployment.Metadata,
			})
		}
	}
	return deployments, nil
}

func (r *BoltDeploymentsRepository) getDeploymentValue(deploymentID common.DeploymentID) (*DeploymentValue, error) {
	deploymentKey, err := r.db.deploymentKey(deploymentID)
	if err != nil {
		slog.Error("BoltDeploymentsRepository.GetDeployment", "deploymentID", deploymentID, "msg", "deploymentKey failed", "error", err.Error())
		return nil, err
	}

	var deploymentValue DeploymentValue
	deploymentStr, err := r.db.get(deploymentKey)
	if err != nil {
		slog.Error("BoltDeploymentsRepository.GetDeployment", "deploymentID", deploymentID, "msg", "get failed", "error", err.Error())
		return nil, err
	}
	if err := json.Unmarshal([]byte(deploymentStr), &deploymentValue); err != nil {
		slog.Error("BoltDeploymentsRepository.GetDeployment", "deploymentID", deploymentID, "msg", "unmarshalling deployment", "deploymentStr", deploymentStr, "error", err.Error())
		return nil, err
	}

	return &deploymentValue, nil
}

func (r *BoltDeploymentsRepository) GetDeployment(deploymentID common.DeploymentID) (*common.Deployment, error) {
	deploymentValue, err := r.getDeploymentValue(deploymentID)
	if err != nil {
		slog.Error("BoltDeploymentsRepository.GetDeployment", "deploymentID", deploymentID, "error", err.Error())
		return nil, err
	}

	loadDriver, err := common.BuildLoadDriver(deploymentValue.LoadDriverConfig)
	if err != nil {
		return nil, err
	}

	return &common.Deployment{
		ID:         deploymentValue.ID,
		LoadName:   deploymentValue.LoadName,
		LoadDriver: loadDriver,
		Status:     deploymentValue.Status,
		Metadata:   deploymentValue.Metadata,
	}, nil
}

func (r *BoltDeploymentsRepository) DeleteByLoad(loadName string) error {
	loadKey, err := r.db.loadKey(loadName)
	if err != nil {
		slog.Error("BoltDeploymentsRepository.DeleteByLoad", "load", loadName, "msg", "loadKey failed", "error", err.Error())
		return err
	}

	loadDeployments, err := r.db.getPrefix(loadKey)
	if err != nil {
		slog.Error("BoltDeploymentsRepository.DeleteByLoad", "loadName", loadName, "msg", "retrieving load deployments", "error", err.Error())
		return err
	}

	// Delete both the load index keys and the deployment entries they point to
	var keys []string
	for loadDeploymentKey, deploymentIDStr := range loadDeployments {
		deploymentID, err := uuid.Parse(deploymentIDStr)
		if err != nil {
			slog.Error("BoltDeploymentsRepository.DeleteByLoad", "loadName", loadName, "msg", "parsing deployment ID", "error", err.Error())
			return err
		}
		deploymentKey, err := r.db.deploymentKey(deploymentID)
		if err != nil {
			slog.Error("BoltDeploymentsRepository.DeleteByLoad", "deploymentID", deploymentID, "msg", "deleting deployment key", "error", err.Error())
			return err
		}

		keys = append(keys, loadDeploymentKey, deploymentKey)
	}

	if err := r.db.deleteKeys(keys...); err != nil {
		slog.Error("BoltDeploymentsRepository.DeleteByLoad", "loadName", loadName, "msg", "running delete transaction", "error", err.Error())
		return err
	}
	return nil
}

func (r *BoltDeploymentsRepository) DeleteDeployment(deploymentID uuid.UUID) error {
	deployment, err := r.GetDeployment(deploymentID)
	if err != nil {
		slog.Error("BoltDeploymentsRepository.DeleteDeployment", "deploymentID", deploymentID, "msg", "GetDeployment failed", "error", err.Error())
		return err
	}
	err = r.DeleteByLoad(deployment.LoadName)
	if err != nil {
		slog.Error("BoltDeploymentsRepository.DeleteDeployment", "deploymentID", deploymentID, "msg", "DeleteByLoad failed", "error", err.Error())
		return err
	}
	return nil
}

func (r *BoltDeploymentsRepository) UpdateStatus(deploymentID common.DeploymentID, status common.DeploymentStatus) error {
	deployment, err := r.getDeploymentValue(deploymentID)
	if err != nil {
		slog.Error("BoltDeploymentsRepository.UpdateStatus", "deploymentID", deploymentID, "msg", "GetDeployment failed", "error", err.Error())
		return err
	}

	// Update status
	deployment.Status = status

	deploymentJson, err := json.Marshal(*deployment)
	if err != nil {
		slog.Error("BoltDeploymentsRepository.UpdateStatus", "deploymentID", deploymentID, "msg", "Marshaling deployment", "error", err.Error())
		return err
	}

	deploymentKey, err := r.db.deploymentKey(deploymentID)
	if err != nil {
		slog.Error("BoltDeploymentsRepository.UpdateStatus", "deploymentID", deploymentID, "msg", "creating deployment key", "error", err.Error())
		return err
	}

	if err := r.db.put(deploymentKey, string(deploymentJson)); err != nil {
		slog.Error("BoltDeploymentsRepository.UpdateStatus", "deploymentID", deploymentID, "msg", "db put", "error", err.Error())
		return err
	}

	return nil
}

func (r *BoltDeploymentsRepository) UpdateMetadata(deploymentID common.DeploymentID, updateFn func(metadataPtr any) error) error {
	deploymentKey, err := r.db.deploymentKey(deploymentID)
	if err != nil {
		slog.Error("BoltDeploymentsRepository.Update", "deploymentID", deploymentID, "msg", "deploymentKey failed", "error", err.Error())
		return err
	}

	return r.db.updateValue(deploymentKey, func(deploymentData []byte) ([]byte, error) {
		var deployment DeploymentValue
		if err := json.Unmarshal(deploymentData, &deployment); err != nil {
			return nil, err
		}
		if err := updateFn(&deployment.Metadata); err != nil {
			return nil, err
		}
		return json.Marshal(deployment)
	})
}

func (r *BoltDeploymentsRepository) GetByLoadAndStatus(loadName string, statusCode common.DeploymentStatusCode) ([]common.Deployment, error) {
	deployments, err := r.GetByLoad(loadName)
	if err != nil {
		return nil, err
	}

	var filtered []common.Deployment
	for _, d := range deployments {
		if d.Status.StatusCode == statusCode {
			filtered = append(filtered, d)
		}
	}
	return filtered, nil
}

func (r *BoltDeploymentsRepository) GetAll() ([]common.Deployment, error) {
	prefix, err := r.db.deploymentsKeyPrefix()
	if err != nil {
		slog.Error("BoltDeploymentsRepository.GetAll", "msg", "getting deployments prefix", "error", err.Error())
		return nil, err
	}

	kvs, err := r.db.getPrefix(prefix)
	if err != nil {
		slog.Error("BoltDeploymentsRepository.GetAll", "msg", "retrieving all deployments", "error", err.Error())
		return nil, err
	}

	var deployments []common.Deployment
	for _, value := range kvs {
		var deployment DeploymentValue
		if err := json.Unmarshal([]byte(value), &deployment); err != nil {
			slog.Error("BoltDeploymentsRepository.GetAll", "msg", "unmarshalling deployment", "error", err.Error())
			return nil, err
		}

		loadDriver, err := common.BuildLoadDriver(deployment.LoadDriverConfig)
		if err != nil {
			slog.Error("BoltDeploymentsRepository.GetAll", "msg", "building load driver", "error", err.Error())
			return nil, err
		}

		deployments = append(deployments, common.Deployment{
			ID:         deployment.ID,
			LoadName:   deployment.LoadName,
			LoadDriver: loadDriver,
			Status:     deployment.Status,
			Metadata:   deployment.Metadata,
		})
	}

	return deployments, nil
}
