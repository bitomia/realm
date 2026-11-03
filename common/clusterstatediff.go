package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/fatih/color"
	"github.com/pmezard/go-difflib/difflib"
	"gopkg.in/yaml.v3"
)

type ChangeKind string

const (
	ChangeAdd    ChangeKind = "add"
	ChangeRemove ChangeKind = "remove"
	ChangeUpdate ChangeKind = "update"
)

type NodeChange struct {
	Name string
	Kind ChangeKind
}

type ClusterStateDiff struct {
	Nodes []NodeChange
}

func (d ClusterStateDiff) IsEmpty() bool {
	return len(d.Nodes) == 0
}

func DiffClusterStates(previous *ClusterState, planned ClusterState) ClusterStateDiff {
	before := map[string]*Node{}
	if previous != nil {
		for _, node := range previous.Nodes {
			before[node.Name] = node
		}
	}
	after := make(map[string]*Node, len(planned.Nodes))
	for _, node := range planned.Nodes {
		after[node.Name] = node
	}

	names := maps.Clone(before)
	maps.Copy(names, after)

	var diff ClusterStateDiff
	for _, name := range slices.Sorted(maps.Keys(names)) {
		prev, inBefore := before[name]
		next, inAfter := after[name]

		switch {
		case !inBefore:
			diff.Nodes = append(diff.Nodes, NodeChange{Name: name, Kind: ChangeAdd})
		case !inAfter:
			diff.Nodes = append(diff.Nodes, NodeChange{Name: name, Kind: ChangeRemove})
		case !next.SameNodeConfig(prev) || !next.SameLoadsConfig(prev):
			diff.Nodes = append(diff.Nodes, NodeChange{Name: name, Kind: ChangeUpdate})
		}
	}
	return diff
}

func PrintClusterStateDiff(diff ClusterStateDiff, previous *ClusterState, planned ClusterState) error {
	before := nodesByName(previous)
	after := nodesByName(&planned)

	for _, change := range diff.Nodes {
		text, err := nodeDiff(change.Name, before[change.Name], after[change.Name])
		if err != nil {
			return err
		}
		printColoredDiff(text)
	}
	return nil
}

func nodesByName(state *ClusterState) map[string]*Node {
	nodes := map[string]*Node{}
	if state != nil {
		for _, node := range state.Nodes {
			nodes[node.Name] = node
		}
	}
	return nodes
}

func nodeDiff(name string, previous *Node, planned *Node) (string, error) {
	a, err := nodeYAML(previous)
	if err != nil {
		return "", err
	}
	b, err := nodeYAML(planned)
	if err != nil {
		return "", err
	}

	from, to := "current/nodes/"+name, "planned/nodes/"+name
	text, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A:        splitLines(a),
		B:        splitLines(b),
		FromFile: from,
		ToFile:   to,
		Context:  3,
	})
	if err != nil {
		return "", fmt.Errorf("failed to diff node %s: %w", name, err)
	}
	return fmt.Sprintf("diff -u -N %s %s\n%s", from, to, text), nil
}

func nodeYAML(node *Node) (string, error) {
	if node == nil {
		return "", nil
	}

	data, err := json.Marshal(node)
	if err != nil {
		return "", fmt.Errorf("failed to marshal node %s: %w", node.Name, err)
	}

	// JSON is valid YAML, decoding it into a node keeps the fields order
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return "", fmt.Errorf("failed to render node %s: %w", node.Name, err)
	}
	dropJSONStyle(&doc)
	sortDriverConfigs(&doc)

	var out bytes.Buffer
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(2)
	if err := encoder.Encode(&doc); err != nil {
		return "", fmt.Errorf("failed to render node %s: %w", node.Name, err)
	}
	if err := encoder.Close(); err != nil {
		return "", fmt.Errorf("failed to render node %s: %w", node.Name, err)
	}
	return out.String(), nil
}

func dropJSONStyle(node *yaml.Node) {
	node.Style = 0
	for _, child := range node.Content {
		dropJSONStyle(child)
	}
}

func sortDriverConfigs(node *yaml.Node) {
	if node.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value == "driver_config" {
				sortMappingKeys(node.Content[i+1])
			}
		}
	}
	for _, child := range node.Content {
		sortDriverConfigs(child)
	}
}

func sortMappingKeys(node *yaml.Node) {
	if node.Kind != yaml.MappingNode {
		return
	}
	pairs := make([][2]*yaml.Node, 0, len(node.Content)/2)
	for i := 0; i+1 < len(node.Content); i += 2 {
		pairs = append(pairs, [2]*yaml.Node{node.Content[i], node.Content[i+1]})
	}
	slices.SortStableFunc(pairs, func(a, b [2]*yaml.Node) int {
		return strings.Compare(a[0].Value, b[0].Value)
	})
	node.Content = node.Content[:0]
	for _, pair := range pairs {
		node.Content = append(node.Content, pair[0], pair[1])
		sortMappingKeys(pair[1])
	}
}

func splitLines(text string) []string {
	lines := strings.SplitAfter(text, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func printColoredDiff(text string) {
	inHeader := true
	for _, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		switch {
		case strings.HasPrefix(line, "@@"):
			inHeader = false
			fmt.Println(color.CyanString(line))
		case inHeader:
			fmt.Println(color.New(color.Bold).Sprint(line))
		case strings.HasPrefix(line, "+"):
			fmt.Println(color.GreenString(line))
		case strings.HasPrefix(line, "-"):
			fmt.Println(color.RedString(line))
		default:
			fmt.Println(line)
		}
	}
}
