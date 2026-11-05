package golang

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"

	"github.com/bitomia/realm/cmd/log"
	"github.com/bitomia/realm/common"
)

func (c *Client) PlanCluster(stateFile string, dry bool, approve func() (bool, error)) error {
	previous, err := common.ReadClusterStateFile(stateFile)
	if errors.Is(err, os.ErrNotExist) {
		previous = nil
	} else if err != nil {
		return err
	}

	planned := c.cfg.GetClusterState()
	diff := common.DiffClusterStates(previous, planned)
	if diff.IsEmpty() {
		log.Success("No changes, cluster state on %s is up to date", stateFile)
		return nil
	}

	if err := common.PrintClusterStateDiff(diff, previous, planned); err != nil {
		return err
	}

	if dry {
		log.Info("Dry run, %s left untouched", stateFile)
		return nil
	}

	approved, err := approve()
	if err != nil {
		return err
	}
	if !approved {
		log.Warn("Plan discarded, %s left untouched", stateFile)
		return nil
	}

	if err := planned.WriteFile(stateFile); err != nil {
		return err
	}

	log.Success("Cluster state planned on %s", stateFile)
	return nil
}

// StartCluster powers on and waits for the nodes, rolls out the planned
// config (see UpdateCluster) and starts all loads.
func (client *Client) StartCluster(ctx context.Context, stateFile string) error {
	if err := client.checkClusterState(stateFile); err != nil {
		return err
	}

	powerOnSeq := client.powerOnNodesSequence(ctx, client.cfg.GetNodes())
	if !isSuccessPowerOnSequence(powerOnSeq) {
		var errs []error
		for _, name := range slices.Sorted(maps.Keys(powerOnSeq)) {
			if result := powerOnSeq[name]; result.stage != nodeStageReady {
				errs = append(errs, fmt.Errorf("%s: %s (expected %s): %w", name, result.stage, nodeStageReady, result.err))
			}
		}
		return fmt.Errorf("nodes drifted: %w", errors.Join(errs...))
	}

	plan, err := client.planUpdate(client.cfg.GetNodes(), client.cfg.GetLoads())
	if err != nil {
		return err
	}
	if err := client.rollout(plan); err != nil {
		return err
	}

	return client.startLoads(client.cfg.GetLoads())
}

func (client *Client) StopCluster(ctx context.Context, stateFile string, wallMessage string, offsetTime uint32) error {
	if err := client.checkClusterState(stateFile); err != nil {
		return err
	}

	loadsStopResult := client.stopLoadsSequence(client.cfg.GetLoads())
	if !isSuccessLoadsStopSequence(loadsStopResult) {
		var errs []error
		for _, name := range slices.Sorted(maps.Keys(loadsStopResult)) {
			if result := loadsStopResult[name]; result.stage != loadStageStopped {
				errs = append(errs, fmt.Errorf("%s: %s (expected %s): %w", name, result.stage, loadStageStopped, result.err))
			}
		}
		return fmt.Errorf("loads drifted: %w", errors.Join(errs...))
	}

	stopSeq := client.shutdownNodesSequence(ctx, client.cfg.GetNodes(), wallMessage, offsetTime)
	if isSuccessShutdownSequence(stopSeq) {
		return nil
	}

	var errs []error
	for _, name := range slices.Sorted(maps.Keys(stopSeq)) {
		if result := stopSeq[name]; result.stage != nodeStageShutDown {
			errs = append(errs, fmt.Errorf("%s: %s (expected %s): %w", name, result.stage, nodeStageShutDown, result.err))
		}
	}
	return fmt.Errorf("nodes drifted: %w", errors.Join(errs...))
}

// UpdateCluster rolls out the planned loads changes on a running cluster.
// Node config changes other than loads are refused, they require 'stop' and 'start'.
func (c *Client) UpdateCluster(ctx context.Context, stateFile string) error {
	if err := c.checkClusterState(stateFile); err != nil {
		return err
	}

	plan, err := c.planUpdate(c.cfg.GetNodes(), c.cfg.GetLoads())
	if err != nil {
		return err
	}
	if plan.isEmpty() {
		log.Success("No changes, cluster is up to date")
		return nil
	}

	if err := c.rollout(plan); err != nil {
		return err
	}

	return c.startLoads(plan.loads)
}

// rollout stops loads to roll out in stop order and loads the planned config.
func (c *Client) rollout(plan updatePlan) error {
	results := c.stopOrderedLoadsSequence(rolloutStopOrder(plan.loads))
	if !isSuccessLoadsStopSequence(results) {
		var errs []error
		for _, name := range slices.Sorted(maps.Keys(results)) {
			if result := results[name]; result.stage != loadStageStopped {
				errs = append(errs, fmt.Errorf("%s: %s (expected %s): %w", name, result.stage, loadStageStopped, result.err))
			}
		}
		return fmt.Errorf("loads drifted: %w", errors.Join(errs...))
	}

	var errs []error
	updateSeqResult := c.updateNodesSequence(plan.nodes)
	if !isSuccessUpdateSequence(updateSeqResult) {
		for _, name := range slices.Sorted(maps.Keys(updateSeqResult)) {
			if result := updateSeqResult[name]; result.stage != nodeStageUpdated {
				errs = append(errs, fmt.Errorf("%s: %s (expected %s): %w", name, result.stage, nodeStageUpdated, result.err))
			}
		}
		return fmt.Errorf("nodes drifted: %w", errors.Join(errs...))
	}

	return nil
}

// startLoads starts the given loads and their dependencies in start order.
func (c *Client) startLoads(loads map[string]*common.Load) error {
	results := c.startLoadsSequence(loads)
	if isSuccessLoadsStartSequence(results) {
		return nil
	}

	var errs []error
	for _, name := range slices.Sorted(maps.Keys(results)) {
		if result := results[name]; result.stage != loadStageRunning {
			errs = append(errs, fmt.Errorf(" %s: %s (expected %s): %w", name, result.stage, loadStageRunning, result.err))
		}
	}

	return fmt.Errorf("loads drifted: %w", errors.Join(errs...))
}

func (c *Client) checkClusterState(stateFile string) error {
	previous, err := common.ReadClusterStateFile(stateFile)
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("no cluster state found on %s: run 'plan' first", stateFile)
	} else if err != nil {
		return err
	}

	planned := c.cfg.GetClusterState()
	diff := common.DiffClusterStates(previous, planned)
	if diff.IsEmpty() {
		return nil
	}

	if err := common.PrintClusterStateDiff(diff, previous, planned); err != nil {
		return err
	}

	return fmt.Errorf("config differs from cluster state on %s: run 'plan' to review and update it", stateFile)
}
