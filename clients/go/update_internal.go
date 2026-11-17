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

// nodeUpdate is the rollout of the planned config on a node
type nodeUpdate struct {
	node *common.Node
	// configured true if node agent already holds a config
	configured bool
	// stale loads are deprovisioned
	stale []string
	// fresh loads are provisioned
	fresh []string
}

type updatePlan struct {
	nodes map[string]nodeUpdate
	loads map[string]*common.Load
}

func (p updatePlan) isEmpty() bool {
	return len(p.nodes) == 0
}

func (client *Client) planUpdate(nodes map[string]*common.Node, loads map[string]*common.Load) (updatePlan, error) {
	plan := updatePlan{nodes: make(map[string]nodeUpdate)}
	nodeNames := slices.Sorted(maps.Keys(nodes))

	var offline []string
	for _, name := range nodeNames {
		if _, err := client.GetVersion(nodes[name].Url); err != nil {
			offline = append(offline, name)
		}
	}
	if len(offline) > 0 {
		return plan, fmt.Errorf("nodes not running: %s: run 'start' first", strings.Join(offline, ", "))
	}

	changed := make(map[string]bool)
	for _, name := range nodeNames {
		node := nodes[name]
		update := nodeUpdate{node: node, configured: true}

		validateErr := client.ValidateNodeConfig(node)
		switch {
		case errors.Is(validateErr, common.ErrNodeAlreadyConfigured), errors.Is(validateErr, common.ErrNodeLoadsMismatch), errors.Is(validateErr, common.ErrNodeLiveConfigMismatch):
		case validateErr == nil:
			// The node is running but not configured yet
			update.configured = false
		case errors.Is(validateErr, common.ErrNodeConfigMismatch):
			return plan, fmt.Errorf("node %s config mismatch: run 'stop' and 'start' to apply it", name)
		default:
			return plan, fmt.Errorf("validating %s failed: %w", name, validateErr)
		}

		current, err := client.GetNodeConfig(node)
		if errors.Is(err, common.ErrNodeNotConfigured) {
			current = &common.NodeConfig{}
		} else if err != nil {
			return plan, fmt.Errorf("reading %s config failed: %w", name, err)
		}

		deployments, err := client.GetLoadsDeployments(node.Url)
		if err != nil {
			return plan, fmt.Errorf("reading %s deployments failed: %w", name, err)
		}
		deployed := make(map[string]bool, len(deployments))
		for _, d := range deployments {
			deployed[d.LoadName] = true
		}

		update.stale, update.fresh = diffNodeLoads(current.Loads, node.Loads, deployed)
		if errors.Is(validateErr, common.ErrNodeAlreadyConfigured) && len(update.stale) == 0 && len(update.fresh) == 0 {
			log.Info("Configuring %s skipped: %s", color.CyanString(name), validateErr.Error())
			continue
		}

		for _, load := range update.fresh {
			changed[load] = true
		}
		plan.nodes[name] = update
	}

	plan.loads = rolloutLoads(loads, changed)
	return plan, nil
}

// updateNode deprovisions the stale loads, loads the planned config on the
// node and provisions the fresh loads out of it.
func (client *Client) updateNode(update nodeUpdate) error {
	node := update.node

	// Stale loads go first: a failed update keeps the config they differ from, so retrying finds them again
	if err := client.deprovisionStaleLoads(update); err != nil {
		return err
	}

	if err := client.LoadNodeConfig(node); err != nil && (!update.configured || !errors.Is(err, common.ErrNodeAlreadyConfigured)) {
		return err
	}

	if err := client.provisionFreshLoads(update); err != nil {
		if !update.configured {
			// A node configured by this update is unconfigured again
			if err := client.UnloadNodeConfig(node); err != nil {
				log.Warn("Rolling back %s config failed: %s", color.CyanString(node.Name), err.Error())
			}
		}
		return err
	}
	return nil
}

func (client *Client) deprovisionStaleLoads(update nodeUpdate) error {
	node := update.node
	if len(update.stale) == 0 {
		return nil
	}

	deployments, err := client.GetLoadsDeployments(node.Url)
	if err != nil {
		return fmt.Errorf("reading deployments failed: %w", err)
	}
	running := make(map[string]bool, len(deployments))
	for _, d := range deployments {
		running[d.LoadName] = running[d.LoadName] || d.DeploymentStatus.StatusCode == common.DeploymentStatusRunning
	}

	for _, name := range update.stale {
		isRunning, deployed := running[name]
		if !deployed {
			continue
		}
		// Loads still on the config were stopped by the rollout, removed ones were not
		if isRunning {
			if err := client.StopLoad(&common.Load{Name: name, Node: node}); err != nil {
				return fmt.Errorf("stopping %s failed: %w", name, err)
			}
		}
		if err := client.DeprovisionLoad(node, name); err != nil {
			return fmt.Errorf("deprovisioning %s failed: %w", name, err)
		}
		log.Success("Deprovisioning %s completed", color.CyanString(name))
	}
	return nil
}

// provisionFreshLoads provisions the fresh loads, deprovisioning the already
// provisioned ones when any of them fails.
func (client *Client) provisionFreshLoads(update nodeUpdate) error {
	node := update.node

	for i, name := range update.fresh {
		if err := client.ProvisionLoad(node, name); err != nil {
			for _, provisioned := range update.fresh[:i] {
				if err := client.DeprovisionLoad(node, provisioned); err != nil {
					log.Warn("Rolling back %s provision failed: %s", color.CyanString(provisioned), err.Error())
				}
			}
			return fmt.Errorf("provisioning %s failed: %w", name, err)
		}
		log.Success("Provisioning %s completed", color.CyanString(name))
	}
	return nil
}

// diffNodeLoads returns the deployed loads to deprovision, which changed or
// left the planned config, and the planned loads to provision, which changed
// or are not deployed yet.
func diffNodeLoads(current []common.LoadConfig, planned []common.LoadConfig, deployed map[string]bool) ([]string, []string) {
	changed := make(map[string]bool)
	for _, name := range changedLoads(current, planned) {
		changed[name] = true
	}
	isPlanned := make(map[string]bool, len(planned))
	for _, load := range planned {
		isPlanned[load.Name] = true
	}

	var stale []string
	for _, name := range slices.Sorted(maps.Keys(deployed)) {
		if !isPlanned[name] || changed[name] {
			stale = append(stale, name)
		}
	}
	var fresh []string
	for _, load := range planned {
		if changed[load.Name] || !deployed[load.Name] {
			fresh = append(fresh, load.Name)
		}
	}
	return stale, fresh
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
