package golang

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/fatih/color"

	"github.com/bitomia/realm/cmd/log"
	"github.com/bitomia/realm/common"
)

type updatePlan struct {
	nodes map[string]*common.Node
	loads map[string]*common.Load
}

func (p updatePlan) isEmpty() bool {
	return len(p.nodes) == 0
}

func (client *Client) planUpdate(nodes map[string]*common.Node, loads map[string]*common.Load) (updatePlan, error) {
	plan := updatePlan{nodes: make(map[string]*common.Node)}
	names := slices.Sorted(maps.Keys(nodes))

	var offline []string
	for _, name := range names {
		if _, err := client.GetVersion(nodes[name].Url); err != nil {
			offline = append(offline, name)
		}
	}
	if len(offline) > 0 {
		return plan, fmt.Errorf("nodes not running: %s: run 'start' first", strings.Join(offline, ", "))
	}

	changed := make(map[string]bool)
	for _, name := range names {
		node := nodes[name]
		err := client.ValidateNodeConfig(node)
		switch {
		case errors.Is(err, common.ErrNodeAlreadyConfigured):
			log.Info("Configuring %s skipped: %s", color.CyanString(name), err.Error())
			continue
		case errors.Is(err, common.ErrNodeLoadsMismatch), err == nil:
			// err == nil means the node is running but not configured yet
		case errors.Is(err, common.ErrNodeConfigMismatch):
			return plan, fmt.Errorf("node %s config: run 'stop' and 'start' to apply it", name)
		default:
			return plan, fmt.Errorf("validating %s failed: %w", name, err)
		}

		current, err := client.GetNodeConfig(node)
		if errors.Is(err, common.ErrNodeNotConfigured) {
			current = &common.NodeConfig{}
		} else if err != nil {
			return plan, fmt.Errorf("reading %s config failed: %w", name, err)
		}

		for _, load := range changedLoads(current.Loads, node.Loads) {
			changed[load] = true
		}
		plan.nodes[name] = node
	}

	plan.loads = rolloutLoads(loads, changed)
	return plan, nil
}

// rolloutLoads returns the loads which changed or depend on a changed one.
func rolloutLoads(loads map[string]*common.Load, changed map[string]bool) map[string]*common.Load {
	rollout := make(map[string]*common.Load)
	for name, load := range loads {
		// StartChain holds the load and all its transitive dependencies
		if slices.ContainsFunc(load.StartChain, func(l *common.Load) bool { return changed[l.Name] }) {
			rollout[name] = load
		}
	}
	return rollout
}

// changedLoads returns the names of the planned loads which are missing or
// differ from the current ones.
func changedLoads(current []common.LoadConfig, planned []common.LoadConfig) []string {
	hashes := make(map[string]common.Hash, len(current))
	for _, load := range current {
		hashes[load.Name] = load.Hash()
	}
	var changed []string
	for _, load := range planned {
		if hash, exists := hashes[load.Name]; !exists || hash != load.Hash() {
			changed = append(changed, load.Name)
		}
	}
	return changed
}

// rolloutStopOrder returns the given loads in stop order, without expanding
// their dependencies: only the rolled out loads are stopped.
func rolloutStopOrder(loads map[string]*common.Load) []*common.Load {
	return slices.DeleteFunc(StopOrder(loads), func(l *common.Load) bool {
		_, rolled := loads[l.Name]
		return !rolled
	})
}
