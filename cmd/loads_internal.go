package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"

	"github.com/dominikbraun/graph"
	"github.com/dominikbraun/graph/draw"

	"github.com/bitomia/realm/config"
	loadsDriver "github.com/bitomia/realm/drivers/loads"
)

func NewGraph(loads config.LoadsConfig) (graph.Graph[string, string], error) {
	g := graph.New(graph.StringHash, graph.Directed(), graph.Acyclic())
	allLoads := config.GetLoads()

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

func generateSVG(loads config.LoadsConfig, outputFile string) error {
	allLoads := config.GetLoads()

	if len(allLoads) == 0 {
		return fmt.Errorf("No loads found")
	}

	g := graph.New(graph.StringHash, graph.Directed(), graph.Acyclic())

	for _, load := range allLoads {
		// Determine node attributes based on driver type
		var fillcolor, color string
		if load.Driver.GetLoadDriverID() == loadsDriver.ProcessDriverID {
			fillcolor = "#50C878"
			color = "#2E7D4E"
		} else {
			fillcolor = "#4A90E2"
			color = "#2E5C8A"
		}

		// Create node label with load name and node name
		label := load.Name
		if load.Node != nil {
			label = fmt.Sprintf("%s\\n(%s)", load.Name, load.Node.Name)
		}

		if err := g.AddVertex(load.Name,
			graph.VertexAttribute("shape", "box"),
			graph.VertexAttribute("style", "rounded,filled"),
			graph.VertexAttribute("fillcolor", fillcolor),
			graph.VertexAttribute("color", color),
			graph.VertexAttribute("fontcolor", "white"),
			graph.VertexAttribute("fontsize", "12"),
			graph.VertexAttribute("label", label),
		); err != nil {
			return fmt.Errorf("failed to add node %s: %w", load.Name, err)
		}
	}

	for _, load := range allLoads {
		for _, dep := range load.DependsOn {
			if err := g.AddEdge(load.Name, dep.Name,
				graph.EdgeAttribute("color", "#666666"),
				graph.EdgeAttribute("penwidth", "2"),
			); err != nil {
				return fmt.Errorf("failed to add edge %s -> %s: %s", dep.Name, load.Name, err.Error())
			}
		}
	}

	var buf bytes.Buffer
	if err := draw.DOT(g, &buf); err != nil {
		return fmt.Errorf("failed to generate DOT: %w", err)
	}

	err := convertDotToSVG(buf.String(), outputFile)
	if err != nil {
		return fmt.Errorf("failed to convert DOT to SVG: %w", err)
	}

	return nil
}

func convertDotToSVG(dotString string, outFile string) error {
	if _, err := exec.LookPath("dot"); err != nil {
		return fmt.Errorf("graphviz 'dot' command not found. Please install graphviz")
	}

	cmd := exec.Command("dot", "-Tsvg")
	var out bytes.Buffer
	cmd.Stdout = &out

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		panic(err)
	}

	stdin.Write([]byte(dotString))
	stdin.Close()

	if err := cmd.Wait(); err != nil {
		panic(err)
	}
	os.WriteFile(outFile, out.Bytes(), 0644)

	return nil
}
