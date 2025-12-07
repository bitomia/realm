package db

import (
	"encoding/json"
	"log/slog"

	"github.com/bitomia/realm/common"
	"github.com/google/uuid"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type EtcdLoadsRepository struct {
	db *DaemonDB
}

func (r *EtcdLoadsRepository) Create(loadName string, pid int, driver common.LoadDriver) (common.DeploymentID, error) {
	deploymentID := uuid.New()

	slog.Info("EtcdLoadsRepository.Create", "deploymentID", deploymentID, "loadName", loadName)

	loadDriverJson, err := json.Marshal(driver)
	if err != nil {
		slog.Error("EtcdLoadsRepository.Create", "deploymentID", deploymentID, "msg", "Marshalling driver", "error", err.Error())
		return uuid.Nil, err
	}

	loadKey, err := r.db.loadDeploymentKey(loadName, deploymentID)
	if err != nil {
		slog.Error("EtcdLoadsRepository.Create", "deploymentID", deploymentID, "msg", "creating load-deployment key", "error", err.Error())
		return uuid.Nil, err
	}
	deploymentKey, err := r.db.deploymentKey(deploymentID)
	if err != nil {
		slog.Error("EtcdLoadsRepository.Create", "deploymentID", deploymentID, "msg", "creating deployment key", "error", err.Error())
		return uuid.Nil, err
	}

	ops := []clientv3.Op{
		clientv3.OpPut(deploymentKey, string(loadDriverJson)),
		clientv3.OpPut(loadKey, deploymentID.String()),
	}

	if _, err := r.db.txn(ops...); err != nil {
		slog.Error("EtcdLoadsRepository.Create", "deploymentID", deploymentID, "loadName", loadName, "msg", "db put", "error", err.Error())
		return uuid.Nil, err
	}

	return deploymentID, nil
}

func (r *EtcdLoadsRepository) GetByLoad(loadName string) ([]common.Deployment, error) {
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
			var deployment common.Deployment
			if err := json.Unmarshal([]byte(kv.Value), &deployment); err != nil {
				slog.Error("EtcdLoadsRepository.GetByLoad", "loadName", loadName, "msg", "key", kv.Key, "unmarshallijng deployment", "error", err.Error())
				return nil, err
			}
			deployments = append(deployments, deployment)
		}
	}
	return deployments, nil
}

func (r *EtcdLoadsRepository) GetDeployment(deploymentID common.DeploymentID) (*common.Deployment, error) {
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

func (r *EtcdLoadsRepository) DeleteByLoad(loadName string) error {
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

func (r *EtcdLoadsRepository) DeleteDeployment(deploymentID uuid.UUID) error {
	deployment, err := r.GetDeployment(deploymentID)
	if err != nil {
		slog.Error("EtcdLoadsRepository.DeleteDeployment", "deploymentID", deploymentID, "msg", "GetDeployment failed", "error", err.Error())
		return err
	}
	err = r.DeleteByLoad(deployment.Load.Name)
	if err != nil {
		slog.Error("EtcdLoadsRepository.DeleteDeployment", "deploymentID", deploymentID, "msg", "DeleteByLoad failed", "error", err.Error())
		return err
	}
	return nil
}
