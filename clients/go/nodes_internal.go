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
	nodeStageNotReady         nodeStartStage = "not ready"
)

func isSuccessPowerOnSequence(stages map[string]nodeStartStage) bool {
	for _, stage := range stages {
		if stage != nodeStageReady {
			return false
		}
	}
	return true
}

// powerOnNodesSequence powers on and waits for the given nodes to be ready,
// Returns the last report start stages, if all nodes are nodeStageReady then it's success
func (client *Client) powerOnNodesSequence(ctx context.Context, nodes map[string]*common.Node) map[string]nodeStartStage {
	stages := make(map[string]nodeStartStage, len(nodes))
	for name := range nodes {
		stages[name] = nodeStagePending
	}

	for _, name := range slices.Sorted(maps.Keys(nodes)) {
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
