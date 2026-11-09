package golang

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/fatih/color"

	"github.com/bitomia/realm/cmd/log"
	"github.com/bitomia/realm/common"
)

type nodePowerStage string
type nodeUpdateStage string

const (
	// power-on stages
	nodeStagePowerOnPending   nodePowerStage = "pending"
	nodeStatePowerOnRequested nodePowerStage = "power-on requested"
	nodeStageReady            nodePowerStage = "ready"
	nodeStageNotReady         nodePowerStage = "not ready"
	nodeStageFailed           nodePowerStage = "failed"

	// shutdown stages
	nodeStageShutdownPending   nodePowerStage = "pending"
	nodeStageShutdownRequested nodePowerStage = "shutdown requested"
	nodeStageShutDown          nodePowerStage = "shut down"
	nodeStageStillRunning      nodePowerStage = "still running"

	// update stages
	nodeStageUpdatePending nodeUpdateStage = "pending"
	nodeStageUpdated       nodeUpdateStage = "updated"
	nodeStageUpdateFailed  nodeUpdateStage = "update failed"
)

type nodePowerSequenceResult struct {
	stage nodePowerStage
	err   error
}

func isSuccessPowerOnSequence(sequenceRes map[string]nodePowerSequenceResult) bool {
	for _, result := range sequenceRes {
		if result.stage != nodeStageReady {
			return false
		}
	}
	return true
}

func isSuccessShutdownSequence(sequenceRes map[string]nodePowerSequenceResult) bool {
	for _, result := range sequenceRes {
		if result.stage != nodeStageShutDown {
			return false
		}
	}
	return true
}

type nodeUpdateSequenceResult struct {
	stage nodeUpdateStage
	err   error
}

func isSuccessUpdateSequence(results map[string]nodeUpdateSequenceResult) bool {
	for _, result := range results {
		if result.err != nil {
			return false
		}
	}
	return true
}

// powerOnNodesSequence powers on and waits for the given nodes to be ready,
// Returns the last report start stages, if all nodes are nodeStageReady then it's success
func (client *Client) powerOnNodesSequence(ctx context.Context, nodes map[string]*common.Node) map[string]nodePowerSequenceResult {
	state := make(map[string]nodePowerSequenceResult, len(nodes))
	for name := range nodes {
		state[name] = nodePowerSequenceResult{stage: nodeStagePowerOnPending}
	}

	for _, name := range slices.Sorted(maps.Keys(nodes)) {
		if err := client.PowerOnNode(nodes[name]); err != nil {
			state[name] = nodePowerSequenceResult{nodeStageFailed, fmt.Errorf("powering on %s failed: %w", name, err)}
			return state
		}
		state[name] = nodePowerSequenceResult{stage: nodeStatePowerOnRequested}
	}

	waitCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	ready := true
	client.WaitNodesReady(waitCtx, nodes, func(n *common.Node, err error) {
		if err == nil {
			state[n.Name] = nodePowerSequenceResult{stage: nodeStageReady}
			return
		}
		if errors.Is(err, context.Canceled) && !ready {
			return
		}
		state[n.Name] = nodePowerSequenceResult{nodeStageNotReady, fmt.Errorf("waiting for %s failed: %w", n.Name, err)}
		if ready {
			ready = false
			cancel()
		}
	})

	return state
}

// shutdownNodesSequence shuts down each node and wait for stop state
// Returns the last report stages, if all nodes are nodeStageShutDown then it's success
func (client *Client) shutdownNodesSequence(ctx context.Context, nodes map[string]*common.Node, wallMessage string, offsetTime uint32) map[string]nodePowerSequenceResult {
	state := make(map[string]nodePowerSequenceResult, len(nodes))
	for name := range nodes {
		state[name] = nodePowerSequenceResult{stage: nodeStageShutdownPending}
	}

	for _, name := range slices.Sorted(maps.Keys(nodes)) {
		if err := client.ShutdownNode(nodes[name], wallMessage, offsetTime); err != nil {
			log.Warn("Shutting down %s failed: %s", color.CyanString(name), err.Error())
			state[name] = nodePowerSequenceResult{nodeStageFailed, fmt.Errorf("shutting down %s failed: %s", name, err.Error())}
			return state
		}
		state[name] = nodePowerSequenceResult{stage: nodeStageShutdownRequested}
	}

	client.WaitNodesStopped(ctx, nodes, func(n *common.Node, err error) {
		if err == nil {
			state[n.Name] = nodePowerSequenceResult{stage: nodeStageShutDown}
		} else {
			state[n.Name] = nodePowerSequenceResult{nodeStageStillRunning, fmt.Errorf("waiting for %s failed: %s", color.CyanString(n.Name), err.Error())}
		}
	})

	return state
}

// updateNodesSequence loads the planned config on the given nodes.
// Returns the last report update stages, if all nodes are nodeStageUpdated then it's success
func (client *Client) updateNodesSequence(nodes map[string]*common.Node) map[string]nodeUpdateSequenceResult {
	results := make(map[string]nodeUpdateSequenceResult, len(nodes))
	for name := range nodes {
		results[name] = nodeUpdateSequenceResult{stage: nodeStageUpdatePending}
	}

	for _, name := range slices.Sorted(maps.Keys(nodes)) {
		if err := client.LoadNodeConfig(nodes[name]); err != nil {
			results[name] = nodeUpdateSequenceResult{nodeStageUpdateFailed, fmt.Errorf("configuring %s failed: %s", name, err.Error())}
			return results
		}
		results[name] = nodeUpdateSequenceResult{stage: nodeStageUpdated}
	}

	return results
}
