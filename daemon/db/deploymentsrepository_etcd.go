package db

import (
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"
	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/bitomia/realm/common"
)

type EtcdDeploymentsRepository struct {
	db *DaemonDB
}

type DeploymentValue struct {
	ID               common.DeploymentID
	LoadName         string
	State            common.DeploymentState
	LoadDriverConfig common.LoadDriverConfig
	Metadata         any
}

func (r *EtcdDeploymentsRepository) Create(loadName string, driver common.LoadDriver, state common.DeploymentState, metadata any) (common.DeploymentID, error) {
	deployment := DeploymentValue{
		ID:               uuid.New(),
		LoadName:         loadName,
		State:            state,
		LoadDriverConfig: driver.GetDriverConfig(),
		Metadata:         metadata,
	}

	slog.Info("EtcdLoadsRepository.Create", "deploymentID", deployment.ID, "loadName", loadName)

	deploymentJson, err := json.Marshal(deployment)
	if err != nil {
		slog.Error("EtcdLoadsRepository.Create", "deploymentID", deployment.ID, "msg", "Marshalling driver", "error", err.Error())
		return uuid.Nil, err
	}

	loadKey, err := r.db.loadDeploymentKey(loadName, deployment.ID)
	if err != nil {
		slog.Error("EtcdLoadsRepository.Create", "deploymentID", deployment.ID, "msg", "creating load-deployment key", "error", err.Error())
		return uuid.Nil, err
	}
	deploymentKey, err := r.db.deploymentKey(deployment.ID)
	if err != nil {
		slog.Error("EtcdLoadsRepository.Create", "deploymentID", deployment.ID, "msg", "creating deployment key", "error", err.Error())
		return uuid.Nil, err
	}

	ops := []clientv3.Op{
		clientv3.OpPut(deploymentKey, string(deploymentJson)),
		clientv3.OpPut(loadKey, deployment.ID.String()),
	}

	if _, err := r.db.txn(ops...); err != nil {
		slog.Error("EtcdLoadsRepository.Create", "deploymentID", deployment.ID, "loadName", loadName, "msg", "db put", "error", err.Error())
		return uuid.Nil, err
	}

	return deployment.ID, nil
}

func (r *EtcdDeploymentsRepository) GetByLoad(loadName string) ([]common.Deployment, error) {
	loadKey, err := r.db.loadKey(loadName)
	if err != nil {
		slog.Error("EtcdLoadsRepository.GetByLoad", "load", loadName, "msg", "loadKey failed", "error", err.Error())
		return nil, err
	}

	loadDeployments, err := r.db.getKey(loadKey)
	if err != nil {
		slog.Error("EtcdLoadsRepository.GetByLoad", "loadName", loadName, "msg", "retrieving load deployments", "error", err.Error())
		return nil, err
	}

	var ops []clientv3.Op
	for _, deploymentIDStr := range loadDeployments {
		deploymentID, err := uuid.Parse(deploymentIDStr)
		if err != nil {
			slog.Error("EtcdLoadsRepository.GetByLoad", "loadName", loadName, "msg", "parsing deployment ID", "error", err.Error())
			return nil, err
		}
		deploymentKey, err := r.db.deploymentKey(deploymentID)
		if err != nil {
			slog.Error("EtcdLoadsRepository.GetByLoad", "deploymentID", deploymentID, "msg", "deleting deployment key", "error", err.Error())
			return nil, err
		}
		ops = append(ops, clientv3.OpGet(deploymentKey))
	}
	txnRes, err := r.db.txn(ops...)
	if txnRes == nil && err == nil {
		return nil, nil
	}
	if !txnRes.Succeeded {
		slog.Error("EtcdLoadsRepository.GetByLoad", "loadName", loadName, "msg", "transaction failed", "error", err.Error())
		return nil, err
	}

	var deployments []common.Deployment
	for _, r := range txnRes.Responses {
		getResp := r.GetResponseRange()
		if getResp == nil {
			continue
		}

		for _, kv := range getResp.Kvs {
			var deployment DeploymentValue
			if err := json.Unmarshal([]byte(kv.Value), &deployment); err != nil {
				slog.Error("EtcdLoadsRepository.GetByLoad", "loadName", loadName, "msg", "unmarshalling deployment", "key", kv.Key, "error", err.Error())
				return nil, err
			}

			if loadDriver, err := common.BuildLoadDriver(deployment.LoadDriverConfig); err != nil {
				return nil, err
			} else {
				deployments = append(deployments, common.Deployment{
					ID:         deployment.ID,
					LoadName:   loadName,
					LoadDriver: loadDriver,
					State:      deployment.State,
					Metadata:   deployment.Metadata,
				})
			}
		}
	}
	return deployments, nil
}

func (r *EtcdDeploymentsRepository) getDeploymentValue(deploymentID common.DeploymentID) (*DeploymentValue, error) {
	deploymentKey, err := r.db.deploymentKey(deploymentID)
	if err != nil {
		slog.Error("EtcdLoadsRepository.GetDeployment", "deploymentID", deploymentID, "msg", "deploymentKey failed", "error", err.Error())
		return nil, err
	}

	var deploymentValue DeploymentValue
	deploymentStr, err := r.db.get(deploymentKey)
	if err := json.Unmarshal([]byte(deploymentStr), &deploymentValue); err != nil {
		slog.Error("EtcdLoadsRepository.GetDeployment", "deploymentID", deploymentID, "msg", "unmarshalling deployment", "deploymentStr", deploymentStr, "error", err.Error())
		return nil, err
	}

	return &deploymentValue, nil
}

func (r *EtcdDeploymentsRepository) GetDeployment(deploymentID common.DeploymentID) (*common.Deployment, error) {
	deploymentValue, err := r.getDeploymentValue(deploymentID)
	if err != nil {
		slog.Error("EtcdLoadsRepository.GetDeployment", "deploymentID", deploymentID, "error", err.Error())
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
		State:      deploymentValue.State,
		Metadata:   deploymentValue.Metadata,
	}, nil
}

func (r *EtcdDeploymentsRepository) DeleteByLoad(loadName string) error {
	loadKey, err := r.db.loadKey(loadName)
	if err != nil {
		slog.Error("EtcdLoadsRepository.DeleteByLoad", "load", loadName, "msg", "loadKey failed", "error", err.Error())
		return err
	}

	loadDeployments, err := r.db.getKey(loadKey)
	if err != nil {
		slog.Error("EtcdLoadsRepository.DeleteByLoad", "loadName", loadName, "msg", "retrieving load deployments", "error", err.Error())
		return err
	}

	var ops []clientv3.Op
	for _, deploymentIDStr := range loadDeployments {
		deploymentID, err := uuid.Parse(deploymentIDStr)
		if err != nil {
			slog.Error("EtcdLoadsRepository.DeleteByLoad", "loadName", loadName, "msg", "parsing deployment ID", "error", err.Error())
			return err
		}
		deploymentKey, err := r.db.deploymentKey(deploymentID)
		if err != nil {
			slog.Error("EtcdLoadsRepository.DeleteByLoad", "deploymentID", deploymentID, "msg", "deleting deployment key", "error", err.Error())
			return err
		}

		ops = append(ops, clientv3.OpDelete(deploymentKey))
	}
	ops = append(ops, clientv3.OpDelete(loadKey, clientv3.WithPrefix()))

	if _, err := r.db.txn(ops...); err != nil {
		slog.Error("EtcdLoadsRepository.DeleteByLoad", "loadName", loadName, "msg", "running delete transaction", "error", err.Error())
		return err
	}
	return nil
}

func (r *EtcdDeploymentsRepository) DeleteDeployment(deploymentID uuid.UUID) error {
	deployment, err := r.GetDeployment(deploymentID)
	if err != nil {
		slog.Error("EtcdLoadsRepository.DeleteDeployment", "deploymentID", deploymentID, "msg", "GetDeployment failed", "error", err.Error())
		return err
	}
	err = r.DeleteByLoad(deployment.LoadName)
	if err != nil {
		slog.Error("EtcdLoadsRepository.DeleteDeployment", "deploymentID", deploymentID, "msg", "DeleteByLoad failed", "error", err.Error())
		return err
	}
	return nil
}

func (r *EtcdDeploymentsRepository) UpdateState(deploymentID common.DeploymentID, state common.DeploymentState) error {
	deployment, err := r.getDeploymentValue(deploymentID)
	if err != nil {
		slog.Error("EtcdLoadsRepository.UpdateState", "deploymentID", deploymentID, "msg", "GetDeployment failed", "error", err.Error())
		return err
	}

	// Update state
	deployment.State = state

	deploymentJson, err := json.Marshal(*deployment)
	if err != nil {
		slog.Error("EtcdLoadsRepository.UpdateState", "deploymentID", deploymentID, "msg", "Marshalling deployment", "error", err.Error())
		return err
	}

	deploymentKey, err := r.db.deploymentKey(deploymentID)
	if err != nil {
		slog.Error("EtcdLoadsRepository.UpdateState", "deploymentID", deploymentID, "msg", "creating deployment key", "error", err.Error())
		return err
	}

	ops := []clientv3.Op{
		clientv3.OpPut(deploymentKey, string(deploymentJson)),
	}

	if _, err := r.db.txn(ops...); err != nil {
		slog.Error("EtcdLoadsRepository.UpdateState", "deploymentID", deploymentID, "msg", "db put", "error", err.Error())
		return err
	}

	return nil
}

func (r *EtcdDeploymentsRepository) GetByLoadAndState(loadName string, state common.DeploymentState) ([]common.Deployment, error) {
	deployments, err := r.GetByLoad(loadName)
	if err != nil {
		return nil, err
	}

	var filtered []common.Deployment
	for _, d := range deployments {
		if d.State == state {
			filtered = append(filtered, d)
		}
	}
	return filtered, nil
}

func (r *EtcdDeploymentsRepository) GetAll() ([]common.Deployment, error) {
	prefix, err := r.db.deploymentsKeyPrefix()
	if err != nil {
		slog.Error("EtcdLoadsRepository.GetAll", "msg", "getting deployments prefix", "error", err.Error())
		return nil, err
	}

	kvs, err := r.db.getKey(prefix)
	if err != nil {
		slog.Error("EtcdLoadsRepository.GetAll", "msg", "retrieving all deployments", "error", err.Error())
		return nil, err
	}

	var deployments []common.Deployment
	for _, value := range kvs {
		var deployment DeploymentValue
		if err := json.Unmarshal([]byte(value), &deployment); err != nil {
			slog.Error("EtcdLoadsRepository.GetAll", "msg", "unmarshalling deployment", "error", err.Error())
			return nil, err
		}

		loadDriver, err := common.BuildLoadDriver(deployment.LoadDriverConfig)
		if err != nil {
			slog.Error("EtcdLoadsRepository.GetAll", "msg", "building load driver", "error", err.Error())
			return nil, err
		}

		deployments = append(deployments, common.Deployment{
			ID:         deployment.ID,
			LoadName:   deployment.LoadName,
			LoadDriver: loadDriver,
			State:      deployment.State,
			Metadata:   deployment.Metadata,
		})
	}

	return deployments, nil
}
