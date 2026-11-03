package golang

import (
	"context"
	"errors"
	"maps"
	"slices"

	"github.com/fatih/color"

	"github.com/bitomia/realm/cmd/log"
	"github.com/bitomia/realm/common"
)

type nodeStartStage string

const (
	nodeStagePending          nodeStartStage = "pending"
	nodeStatePowerOnRequested nodeStartStage = "power-on requested"
	nodeStageReady            nodeStartStage = "ready"
	nodeStageConfigured       nodeStartStage = "configured"

	nodeStageNotReady       nodeStartStage = "not ready"
	nodeStageInvalidConfig  nodeStartStage = "invalid config"
	nodeStageConfigMismatch nodeStartStage = "config mismatch"
)

func isSuccessStartSequence(stages map[string]nodeStartStage) bool {
	for _, stage := range stages {
		if stage != nodeStageConfigured {
			return false
		}
	}
	return true
}

// startNodesSequence powers on, waits for and configures the given nodes,
// stopping at the first failure.
// Returns the last report start stages, if all nodes are nodeStageConfigured then it's success
func (client *Client) startNodesSequence(ctx context.Context, nodes map[string]*common.Node) map[string]nodeStartStage {
	stages := make(map[string]nodeStartStage, len(nodes))
	for name := range nodes {
		stages[name] = nodeStagePending
	}

	names := slices.Sorted(maps.Keys(nodes))

	for _, name := range names {
		if err := client.PowerOnNode(nodes[name]); err != nil {
			log.Warn("Powering on %s failed: %s", color.CyanString(name), err.Error())
			return stages
		}
		stages[name] = nodeStatePowerOnRequested
	}

	waitCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	ready := true
	client.WaitNodesReady(waitCtx, nodes, func(n *common.Node, err error) {
		if err == nil {
			stages[n.Name] = nodeStageReady
			return
		}
		// Nodes whose wait was cancelled by another node's failure keep their stage
		if errors.Is(err, context.Canceled) && !ready {
			return
		}
		stages[n.Name] = nodeStageNotReady
		log.Warn("Waiting for %s failed: %s", color.CyanString(n.Name), err.Error())
		if ready {
			ready = false
			cancel()
		}
	})
	if !ready {
		return stages
	}

	toLoad := make([]string, 0, len(names))
	for _, name := range names {
		if err := client.ValidateNodeConfig(nodes[name]); err == nil {
			toLoad = append(toLoad, name)
		} else if errors.Is(err, common.ErrNodeAlreadyConfigured) {
			stages[name] = nodeStageConfigured
			log.Info("Loading %s skipped: %s", color.CyanString(name), err.Error())
		} else if errors.Is(err, common.ErrNodeLoadsMismatch) {
			toLoad = append(toLoad, name)
			log.Info("Loading %s reprovisions loads: %s", color.CyanString(name), err.Error())
		} else if errors.Is(err, common.ErrNodeConfigMismatch) {
			stages[name] = nodeStageConfigMismatch
			log.Warn("Loading %s aborted: %s", color.CyanString(name), err.Error())
			return stages
		} else {
			stages[name] = nodeStageInvalidConfig
			log.Warn("Validating %s failed: %s", color.CyanString(name), err.Error())
			return stages
		}
	}

	for _, name := range toLoad {
		if err := client.LoadNodeConfig(nodes[name]); err != nil {
			log.Warn("Loading %s failed: %s", color.CyanString(name), err.Error())
			return stages
		}
		stages[name] = nodeStageConfigured
	}

	return stages
}

type nodePowerOffStage string

const (
	nodeStagePowerOffRequested nodePowerOffStage = "power-off requested"
	nodeStagePoweredOff        nodePowerOffStage = "powered off"
	nodeStageStillRunning      nodePowerOffStage = "still running"
)

func isSuccessPowerOffSequence(stages map[string]nodePowerOffStage) bool {
	for _, stage := range stages {
		if stage != nodeStagePoweredOff {
			return false
		}
	}
	return true
}

// powerOffNodesSequence checks that no running node drifted from its config
// (loads are not checked) and only then powers off and waits for all of them.
// Returns the last report stages, if all nodes are nodeStagePoweredOff then it's success
func (client *Client) powerOffNodesSequence(ctx context.Context, nodes map[string]*common.Node) map[string]nodePowerOffStage {
	stages := make(map[string]nodePowerOffStage, len(nodes))

	for _, name := range slices.Sorted(maps.Keys(nodes)) {
		if err := client.PowerOffNode(nodes[name]); err != nil {
			log.Warn("Powering off %s failed: %s", color.CyanString(name), err.Error())
			return stages
		}
		stages[name] = nodeStagePowerOffRequested
	}

	client.WaitNodesStopped(ctx, nodes, func(n *common.Node, err error) {
		if err == nil {
			stages[n.Name] = nodeStagePoweredOff
		} else {
			stages[n.Name] = nodeStageStillRunning
			log.Warn("Waiting for %s failed: %s", color.CyanString(n.Name), err.Error())
		}
	})

	return stages
}

type nodeStopStage string

const (
	nodeStageRunning      nodeStopStage = "running"
	nodeStageUnconfigured nodeStopStage = "unconfigured"
	nodeStageShuttingDown nodeStopStage = "shutting down"
	nodeStageStopped      nodeStopStage = "stopped"
)

func isSuccessStopSequence(stages map[string]nodeStopStage) bool {
	for _, stage := range stages {
		if stage != nodeStageStopped {
			return false
		}
	}
	return true
}

// stopNodesSequence unconfigures, shuts down and waits for the given nodes to
// stop, powering off the ones still running after their stop timeout.
// Returns the last report stop stages, if all nodes are nodeStageStopped then it's success
func (client *Client) stopNodesSequence(ctx context.Context, nodes map[string]*common.Node) map[string]nodeStopStage {
	stages := make(map[string]nodeStopStage, len(nodes))
	running := make(map[string]*common.Node, len(nodes))
	for _, name := range slices.Sorted(maps.Keys(nodes)) {
		if _, err := client.GetVersion(nodes[name].Url); err != nil {
			stages[name] = nodeStageStopped
			log.Info("Shutting down %s skipped: node not running", color.CyanString(name))
		} else {
			stages[name] = nodeStageRunning
			running[name] = nodes[name]
		}
	}

	names := slices.Sorted(maps.Keys(running))

	for _, name := range names {
		if err := client.UnloadNodeConfig(running[name]); err == nil {
			log.Success("Unloading %s completed", color.CyanString(name))
		} else if errors.Is(err, common.ErrNodeNotConfigured) {
			log.Info("Unloading %s skipped: %s", color.CyanString(name), err.Error())
		} else {
			log.Warn("Unloading %s failed: %s", color.CyanString(name), err.Error())
			return stages
		}
		stages[name] = nodeStageUnconfigured
	}

	for _, name := range names {
		if err := client.ShutdownNode(running[name], "", 0); err != nil {
			log.Warn("Shutting down %s failed: %s", color.CyanString(name), err.Error())
			return stages
		}
		stages[name] = nodeStageShuttingDown
	}

	var timedOut []string
	client.WaitNodesStopped(ctx, running, func(n *common.Node, err error) {
		if err == nil {
			stages[n.Name] = nodeStageStopped
		} else {
			log.Warn("Waiting for %s failed: %s", color.CyanString(n.Name), err.Error())
			timedOut = append(timedOut, n.Name)
		}
	})
	if ctx.Err() != nil {
		return stages
	}

	slices.Sort(timedOut)
	for _, name := range timedOut {
		if err := client.PowerOffNode(running[name]); err != nil {
			log.Warn("Powering off %s failed: %s", color.CyanString(name), err.Error())
			continue
		}
		stages[name] = nodeStageStopped
		log.Info("Powering off %s completed", color.CyanString(name))
	}

	return stages
}
