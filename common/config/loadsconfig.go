package config

import (
	"fmt"
	"slices"

	"github.com/dominikbraun/graph"

	"github.com/bitomia/realm/common"
)

var (
	loadsConfig      map[string]*common.Load = make(map[string]*common.Load)
	loadsConfigGraph graph.Graph[string, string]
)

func ResetLoadsConfig() {
	loadsConfig = make(map[string]*common.Load)
	loadsConfigGraph = nil
}

func newLoadConfig(loadName string, node *common.NodeConfig, driver common.LoadDriver) (*common.Load, error) {
	if _, exists := loadsConfig[loadName]; exists {
		return nil, fmt.Errorf("Load name not unique")
	}
	loadsConfig[loadName] = &common.Load{Name: loadName, Node: node, Driver: driver}
	return loadsConfig[loadName], nil
}

func newLoadsConfigGraph() error {
	loadsConfigGraph = graph.New(graph.StringHash, graph.Directed(), graph.Acyclic(), graph.PreventCycles())

	allLoads := GetLoadsFromConfig()
	for _, load := range allLoads {
		if err := loadsConfigGraph.AddVertex(load.Name); err != nil {
			return fmt.Errorf("failed to add node %s: %s", load.Name, err.Error())
		}
	}

	for _, load := range allLoads {
		for _, dep := range load.DependsOn {
			if err := loadsConfigGraph.AddEdge(load.Name, dep.Name); err != nil {
				return fmt.Errorf("failed to add edge %s -> %s: %s", dep.Name, load.Name, err.Error())
			}
		}
	}

	return nil
}

func GetLoadsFromConfig(loadsFilter ...string) map[string]*common.Load {
	loads := make(map[string]*common.Load)
	for _, load := range loadsConfig {
		if len(loadsFilter) == 0 || slices.Contains(loadsFilter, load.Name) {
			loads[load.Name] = load
		}
	}
	return loads
}

func GetLoadsConfigGraph() graph.Graph[string, string] {
	return loadsConfigGraph
}
