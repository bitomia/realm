package dto

import "github.com/bitomia/realm/common"

type LoadInfoResponse struct {
	Name   string `json:"name"`
	Node   string `json:"node"`
	Driver string `json:"driver"`
}

type LoadsInfoResponse []LoadInfoResponse

func NewLoadsInfoResponseDTO(loads map[string]*common.Load) LoadsInfoResponse {
	var loadsInfoRes LoadsInfoResponse
	for _, l := range loads {
		loadsInfoRes = append(loadsInfoRes, LoadInfoResponse{
			Name:   l.Name,
			Node:   l.Node.Name,
			Driver: string(l.Driver.GetLoadDriverID()),
		})
	}
	return loadsInfoRes
}
