package dto

import "github.com/bitomia/realm/common"

type LoadDeployment struct {
	LoadName     string                 `json:"load_name"`
	DeploymentId string                 `json:"deployment_id"`
	State        common.DeploymentState `json:"state"`
	Driver       string                 `json:"driver"`
}

type LoadsDeployments []LoadDeployment

type LoadInfo struct {
	Name   string `json:"name"`
	Node   string `json:"node"`
	Driver string `json:"driver"`
}

type LoadsInfo []LoadInfo

func NewLoadsInfoDTO(loads map[string]*common.Load) LoadsInfo {
	var loadsInfoRes LoadsInfo
	for _, l := range loads {
		loadsInfoRes = append(loadsInfoRes, LoadInfo{
			Name:   l.Name,
			Node:   l.Node.Name,
			Driver: string(l.Driver.GetLoadDriverID()),
		})
	}
	return loadsInfoRes
}
