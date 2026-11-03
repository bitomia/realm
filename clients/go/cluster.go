package golang

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"

	"github.com/fatih/color"

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

func (c *Client) StartCluster(ctx context.Context, stateFile string) int {
	if err := c.checkClusterState(stateFile); err != nil {
		log.Fatal("%s", err)
		return 1
	}

	startSeq := c.startNodesSequence(ctx, c.cfg.GetNodes())
	if !isSuccessStartSequence(startSeq) {
		log.Error("Nodes drifted")
		for _, name := range slices.Sorted(maps.Keys(startSeq)) {
			if stage := startSeq[name]; stage == nodeStageConfigured {
				log.Success(" %s: %s", color.CyanString(name), stage)
			} else {
				log.Warn(" %s: %s (expected %s)", color.CyanString(name), stage, nodeStageConfigured)
			}
		}
		return 1
	}

	loadsSeq := c.startLoadsSequence(c.cfg.GetLoads())
	if !isSuccessLoadsStartSequence(loadsSeq) {
		log.Error("Loads drifted")
		for _, name := range slices.Sorted(maps.Keys(loadsSeq)) {
			if stage := loadsSeq[name]; stage == loadStageRunning {
				log.Success(" %s: %s", color.CyanString(name), stage)
			} else {
				log.Warn(" %s: %s (expected %s)", color.CyanString(name), stage, loadStageRunning)
			}
		}
		return 1
	}

	return 0
}

func (client *Client) StopCluster(ctx context.Context, stateFile string) int {
	if err := client.checkClusterState(stateFile); err != nil {
		log.Fatal("%s", err)
		return 1
	}

	loadsSeq := client.stopLoadsSequence(client.cfg.GetLoads())
	if !isSuccessLoadsStopSequence(loadsSeq) {
		log.Error("Loads drifted")
		for _, name := range slices.Sorted(maps.Keys(loadsSeq)) {
			if stage := loadsSeq[name]; stage == loadStageStopped {
				log.Success(" %s: %s", color.CyanString(name), stage)
			} else {
				log.Warn(" %s: %s (expected %s)", color.CyanString(name), stage, loadStageStopped)
			}
		}
		return 1
	}

	stopSeq := client.powerOffNodesSequence(ctx, client.cfg.GetNodes())
	if isSuccessPowerOffSequence(stopSeq) {
		return 0
	}

	log.Error("Nodes drifted")
	for _, name := range slices.Sorted(maps.Keys(stopSeq)) {
		if stage := stopSeq[name]; stage == nodeStagePoweredOff {
			log.Success(" %s: %s", color.CyanString(name), stage)
		} else {
			log.Warn(" %s: %s (expected %s)", color.CyanString(name), stage, nodeStagePoweredOff)
		}
	}

	return 1
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
