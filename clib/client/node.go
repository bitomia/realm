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

//export GetNodesState
func GetNodesState() *C.char {
	client := clientPkg.NewClient()
	nodes := clientPkg.GetNodes()

	states := make([]dto.NodeStateResponse, 0, len(nodes))
	for _, node := range nodes {
		status, err := client.GetNodeState(node.Url)
		if err != nil || status == nil {
			continue
		}
		states = append(states, *status)
	}

	b, err := json.Marshal(states)
	if err != nil {
		return MakeCString(common.ToJsonCString(err))
	}
	return MakeCString(string(b))
}
