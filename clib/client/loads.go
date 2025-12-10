package main

import "C"

import (
	"encoding/json"
	"fmt"

	"github.com/dominikbraun/graph"

	"github.com/bitomia/realm/clib/common"
	clientPkg "github.com/bitomia/realm/cmd/client"
	"github.com/bitomia/realm/common/config"
	"github.com/bitomia/realm/internal"
	"github.com/bitomia/realm/common/dto"
)

type LoadOpResult struct {
	Node    string `json:"node"`
	Load    string `json:"load"`
	Error   string `json:"error"`
	Success bool   `json:"success"`
}

//export PlanLoads
func PlanLoads() *C.char {
	loads := config.GetLoadsRepository()
	if len(loads) == 0 {
		return MakeCString(common.ToJsonCString(dto.ErrorResponse{Message: "No loads found"}))
	}

	client := clientPkg.NewClient()
	loadsResults := make([]LoadOpResult, 0, len(loads))

	for _, load := range loads {
		if err := client.PlanLoad(load); err != nil {
			loadsResults = append(loadsResults, LoadOpResult{Node: load.Node.Name, Load: load.Name, Success: false, Error: err.Error()})
		} else {
			loadsResults = append(loadsResults, LoadOpResult{Node: load.Node.Name, Load: load.Name, Success: true})
		}
	}

	loadsStatesJson, err := json.Marshal(loadsResults)
	if err != nil {
		return MakeCString(common.ToJsonCString(err))
	}

	return MakeCString(string(loadsStatesJson))
}

//export StartLoads
func StartLoads() *C.char {
	cfg := config.Get()

	g, err := internal.NewGraph(cfg.Loads)
	if err != nil {
		return MakeCString(common.ToJsonCString(dto.ErrorResponse{Message: fmt.Sprintf("Error building graph %s", err.Error())}))
	}

	loads := config.GetLoadsRepository()
	if len(loads) == 0 {
		return MakeCString(common.ToJsonCString(dto.ErrorResponse{Message: "No loads found"}))
	}

	client := clientPkg.NewClient()
	loaded := make(map[string]bool)
	loadsResults := make([]LoadOpResult, 0, len(loads))

	for _, l := range loads {
		var pendingLoads []string

		graph.DFS(g, l.Name, func(value string) bool {
			pendingLoads = append(pendingLoads, l.Name)
			return true
		})

		for i := len(pendingLoads) - 1; i >= 0; i-- {
			load := pendingLoads[i]

			if _, exists := loaded[load]; !exists {
				loaded[load] = true
				loadRun := loads[load]

				if err := client.StartLoad(loadRun); err != nil {
					loadsResults = append(loadsResults, LoadOpResult{Node: l.Node.Name, Load: l.Name, Success: false, Error: err.Error()})
				} else {
					loadsResults = append(loadsResults, LoadOpResult{Node: l.Node.Name, Load: l.Name, Success: true})
				}
			}
		}
	}

	loadsStatesJson, err := json.Marshal(loadsResults)
	if err != nil {
		return MakeCString(common.ToJsonCString(err))
	}

	return MakeCString(string(loadsStatesJson))
}

//export StopLoads
func StopLoads() *C.char {
	cfg := config.Get()

	g, err := internal.NewGraph(cfg.Loads)
	if err != nil {
		return MakeCString(common.ToJsonCString(dto.ErrorResponse{Message: fmt.Sprintf("Error building graph %s", err.Error())}))
	}

	loads := config.GetLoadsRepository()
	if len(loads) == 0 {
		return MakeCString(common.ToJsonCString(dto.ErrorResponse{Message: "No loads found"}))
	}

	client := clientPkg.NewClient()
	stopped := make(map[string]bool)
	loadsResults := make([]LoadOpResult, 0, len(loads))

	for _, l := range loads {
		var pendingLoads []string

		graph.DFS(g, l.Name, func(value string) bool {
			pendingLoads = append([]string{l.Name}, pendingLoads...)
			return true
		})

		for i := len(pendingLoads) - 1; i >= 0; i-- {
			load := pendingLoads[i]

			if _, exists := stopped[load]; !exists {
				stopped[load] = true
				loadStop := loads[load]

				if err := client.StopLoad(loadStop); err != nil {
					loadsResults = append(loadsResults, LoadOpResult{Node: l.Node.Name, Load: l.Name, Success: false, Error: err.Error()})
				} else {
					loadsResults = append(loadsResults, LoadOpResult{Node: l.Node.Name, Load: l.Name, Success: true})
				}
			}
		}
	}

	loadsStatesJson, err := json.Marshal(loadsResults)
	if err != nil {
		return MakeCString(common.ToJsonCString(err))
	}

	return MakeCString(string(loadsStatesJson))
}
