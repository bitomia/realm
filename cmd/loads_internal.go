package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"

	"github.com/awalterschulze/gographviz"

	"github.com/bitomia/realm/internal"
	"github.com/bitomia/realm/internal/config"
)

func generateSVG(loads config.LoadsConfig, outputFile string) error {
	allLoads := loads.GetLoads()

	if len(allLoads) == 0 {
		return fmt.Errorf("No loads found")
	}

	graph := gographviz.NewGraph()
	if err := graph.SetName("LoadsDependencyGraph"); err != nil {
		return fmt.Errorf("failed to set graph name: %w", err)
	}
	if err := graph.SetDir(true); err != nil {
		return fmt.Errorf("failed to set graph direction: %w", err)
	}

	graph.AddAttr("LoadsDependencyGraph", "rankdir", "LR")
	graph.AddAttr("LoadsDependencyGraph", "bgcolor", "white")
	graph.AddAttr("LoadsDependencyGraph", "nodesep", "0.5")
	graph.AddAttr("LoadsDependencyGraph", "ranksep", "1.0")

	for _, load := range allLoads {
		nodeName := fmt.Sprintf(`"%s"`, load.Name)

		// Determine node attributes based on driver type
		attrs := make(map[string]string)
		attrs["shape"] = "\"box\""
		attrs["style"] = "\"rounded,filled\""

		if load.Driver.GetDriverType() == internal.ProcessDriverType {
			attrs["fillcolor"] = "\"#50C878\""
			attrs["color"] = "\"#2E7D4E\""
		} else {
			attrs["fillcolor"] = "\"#4A90E2\""
			attrs["color"] = "\"#2E5C8A\""
		}

		attrs["fontcolor"] = "\"white\""
		attrs["fontsize"] = "12"

		// Create node label with load name and node name
		label := load.Name
		if load.Node != nil {
			label = fmt.Sprintf("%s\\n(%s)", load.Name, load.Node.Name)
		}
		attrs["label"] = fmt.Sprintf(`"%s"`, label)

		if err := graph.AddNode("LoadsDependencyGraph", nodeName, attrs); err != nil {
			return fmt.Errorf("failed to add node %s: %w", load.Name, err)
		}
	}

	for _, load := range allLoads {
		for _, dep := range load.DependsOn {
			fromNode := fmt.Sprintf(`"%s"`, load.Name)
			toNode := fmt.Sprintf(`"%s"`, dep.Name)

			edgeAttrs := make(map[string]string)
			edgeAttrs["color"] = "\"#666666\""
			edgeAttrs["penwidth"] = "2"

			if err := graph.AddEdge(fromNode, toNode, true, edgeAttrs); err != nil {
				return fmt.Errorf("failed to add edge from %s to %s: %w", load.Name, dep.Name, err)
			}
		}
	}

	dotString := graph.String()

	err := convertDotToSVG(dotString, outputFile)
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
