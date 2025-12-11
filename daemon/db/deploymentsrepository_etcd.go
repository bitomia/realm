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
	LoadDriverConfig common.LoadDriverConfig
	Metadata         any
}

func (r *EtcdDeploymentsRepository) Create(loadName string, driver common.LoadDriver, metadata any) (common.DeploymentID, error) {
	deployment := DeploymentValue{
		ID:               uuid.New(),
		LoadName:         loadName,
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
					Metadata:   deployment.Metadata,
				})
			}
		}
	}
	return deployments, nil
}

func (r *EtcdDeploymentsRepository) GetDeployment(deploymentID common.DeploymentID) (*common.Deployment, error) {
	deploymentKey, err := r.db.deploymentKey(deploymentID)
	if err != nil {
		slog.Error("EtcdLoadsRepository.GetDeployment", "deploymentID", deploymentID, "msg", "deploymentKey failed", "error", err.Error())
		return nil, err
	}

	var deployment common.Deployment
	deploymentStr, err := r.db.get(deploymentKey)
	if err := json.Unmarshal([]byte(deploymentStr), &deployment); err != nil {
		slog.Error("EtcdLoadsRepository.GetDeployment", "deploymentID", deploymentID, "msg", "unmarshalling deployment", "deploymentStr", deploymentStr, "error", err.Error())
		return nil, err
	}

	return &deployment, nil
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
	ops = append(ops, clientv3.OpDelete(loadKey))

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
