package main

/*
#include <stdlib.h>
*/
import "C"

import (
	"encoding/json"

	"github.com/bitomia/realm/clib/common"
	clientPkg "github.com/bitomia/realm/cmd/client"
	"github.com/bitomia/realm/internal/dto"
)

type NodeState struct {
	Name  string                `json:"name"`
	Url   string                `json:"url"`
	MAC   *string               `json:"mac,omitempty"`
	State dto.NodeStateResponse `json:"state"`
}

//export GetNodesState
func GetNodesState() *C.char {
	client := clientPkg.NewClient()
	nodes := clientPkg.GetNodes()

	states := make([]NodeState, 0, len(nodes))

	for _, node := range nodes {
		state, err := client.GetNodeState(node.Url)
		if err != nil || state == nil {
			continue
		}
		states = append(states, NodeState{Name: node.Name, Url: node.Url, MAC: node.MAC, State: *state})
	}

	statesJson, err := json.Marshal(states)
	if err != nil {
		return MakeCString(common.ToJsonCString(err))
	}
	return MakeCString(string(statesJson))
}
