package internal

import (
	"fmt"

	"github.com/dominikbraun/graph"

	"github.com/bitomia/realm/common/config"
)

func NewGraph(loads config.LoadsConfig) (graph.Graph[string, string], error) {
	g := graph.New(graph.StringHash, graph.Directed(), graph.Acyclic())
	allLoads := config.GetLoadsRepository()

	for _, load := range allLoads {
		if err := g.AddVertex(load.Name); err != nil {
			return nil, fmt.Errorf("failed to add node %s: %s", load.Name, err.Error())
		}
	}

	for _, load := range allLoads {
		for _, dep := range load.DependsOn {
			if err := g.AddEdge(load.Name, dep.Name); err != nil {
				return nil, fmt.Errorf("failed to add edge %s -> %s: %s", dep.Name, load.Name, err.Error())
			}
		}
	}

	return g, nil
}
