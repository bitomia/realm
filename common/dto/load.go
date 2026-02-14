package dto

import "github.com/bitomia/realm/common"

type LoadDeployment struct {
	LoadName         string                  `json:"load_name"`
	DeploymentId     string                  `json:"deployment_id"`
	DeploymentStatus common.DeploymentStatus `json:"deployment_status"`
	Driver           string                  `json:"driver"`
	DriverConfig     any                     `json:"driver_config"`
	Metadata         any                     `json:"metadata,omitempty"`
}

type LoadsDeployments []LoadDeployment

type LoadInfo struct {
	Name         string `json:"name"`
	Node         string `json:"node"`
	Driver       string `json:"driver"`
	DriverConfig any    `json:"driver_config"`
}

type LoadsInfo []LoadInfo

type ProvisionLoadInfo struct {
	DeploymentId string `json:"deployment_id"`
}
