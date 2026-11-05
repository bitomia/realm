package golang

import (
	"fmt"

	"github.com/fatih/color"

	"github.com/bitomia/realm/cmd/log"
	"github.com/bitomia/realm/common"
)

type loadStage string

const (
	// start tages
	loadStageStartPending loadStage = "pending"
	loadStageRunning      loadStage = "running"
	loadStageStartFailed  loadStage = "start failed"

	// stop stages
	loadStageStopPending loadStage = "pending"
	loadStageStopped     loadStage = "stopped"
	loadStageStopFailed  loadStage = "stop failed"
)

type loadSequenceResult struct {
	stage loadStage
	err   error
}

func isSuccessLoadsStartSequence(results map[string]loadSequenceResult) bool {
	for _, result := range results {
		if result.stage != loadStageRunning {
			return false
		}
	}
	return true
}

func isSuccessLoadsStopSequence(results map[string]loadSequenceResult) bool {
	for _, result := range results {
		if result.stage != loadStageStopped {
			return false
		}
	}
	return true
}

// startLoadsSequence starts the given loads following their start chains
// Returns the last report start stages, if all loads are loadStageRunning then it's success
func (client *Client) startLoadsSequence(loads map[string]*common.Load) map[string]loadSequenceResult {
	return client.startOrderedLoadsSequence(StartOrder(loads))
}

// startOrderedLoadsSequence starts the given loads in the given order
func (client *Client) startOrderedLoadsSequence(order []*common.Load) map[string]loadSequenceResult {
	results := make(map[string]loadSequenceResult, len(order))
	chained := make(map[string]*common.Load, len(order))
	for _, l := range order {
		results[l.Name] = loadSequenceResult{stage: loadStageStartPending}
		chained[l.Name] = l
	}

	for _, state := range client.GetLoadsStates(chained) {
		if state.Err == nil && state.Status.StatusCode == common.DeploymentStatusRunning {
			results[state.Load.Name] = loadSequenceResult{stage: loadStageRunning}
		}
	}

	for _, l := range order {
		if results[l.Name].stage == loadStageRunning {
			log.Info("Starting %s skipped: load already running", color.CyanString(l.Name))
			continue
		}
		if err := client.StartLoad(l); err != nil {
			results[l.Name] = loadSequenceResult{loadStageStartFailed, fmt.Errorf("starting %s failed: %s", l.Name, err.Error())}
			return results
		}
		results[l.Name] = loadSequenceResult{stage: loadStageRunning}
		log.Success("Starting %s completed", color.CyanString(l.Name))
	}

	return results
}

// stopLoadsSequence stops the given loads following their stop chains,
// Returns the last report stop stages, if all loads are loadStageStopped then it's success
func (client *Client) stopLoadsSequence(loads map[string]*common.Load) map[string]loadSequenceResult {
	return client.stopOrderedLoadsSequence(StopOrder(loads))
}

// stopOrderedLoadsSequence stops the given loads in the given order,
func (client *Client) stopOrderedLoadsSequence(order []*common.Load) map[string]loadSequenceResult {
	results := make(map[string]loadSequenceResult, len(order))
	chained := make(map[string]*common.Load, len(order))
	for _, l := range order {
		results[l.Name] = loadSequenceResult{stage: loadStageStopPending}
		chained[l.Name] = l
	}

	for _, state := range client.GetLoadsStates(chained) {
		if state.Err != nil {
			results[state.Load.Name] = loadSequenceResult{stage: loadStageStopped}
			log.Info("Stopping %s skipped: node not reachable: %s", color.CyanString(state.Load.Name), state.Err.Error())
		} else if !state.Provisioned {
			// Nothing deployed yet, e.g. a load rolled out on a fresh node
			results[state.Load.Name] = loadSequenceResult{stage: loadStageStopped}
		} else if state.Status.StatusCode != common.DeploymentStatusRunning {
			results[state.Load.Name] = loadSequenceResult{stage: loadStageStopped}
			log.Info("Stopping %s skipped: load not running", color.CyanString(state.Load.Name))
		}
	}

	for _, l := range order {
		if results[l.Name].stage == loadStageStopped {
			continue
		}
		if err := client.StopLoad(l); err != nil {
			results[l.Name] = loadSequenceResult{stage: loadStageStopFailed, err: fmt.Errorf("stopping %s failed: %s", l.Name, err.Error())}
			return results
		}
		results[l.Name] = loadSequenceResult{stage: loadStageStopped}
		log.Success("Stopping %s completed", color.CyanString(l.Name))
	}

	return results
}
