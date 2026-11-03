package golang

import (
	"github.com/fatih/color"

	"github.com/bitomia/realm/cmd/log"
	"github.com/bitomia/realm/common"
)

type loadStartStage string

const (
	loadStagePending     loadStartStage = "pending"
	loadStageRunning     loadStartStage = "running"
	loadStageStartFailed loadStartStage = "start failed"
)

func isSuccessLoadsStartSequence(stages map[string]loadStartStage) bool {
	for _, stage := range stages {
		if stage != loadStageRunning {
			return false
		}
	}
	return true
}

// startLoadsSequence starts the given loads following their start chains
// Returns the last report start stages, if all loads are loadStageRunning then it's success
func (client *Client) startLoadsSequence(loads map[string]*common.Load) map[string]loadStartStage {
	return client.startOrderedLoadsSequence(StartOrder(loads))
}

// startOrderedLoadsSequence starts the given loads in the given order
func (client *Client) startOrderedLoadsSequence(order []*common.Load) map[string]loadStartStage {
	stages := make(map[string]loadStartStage, len(order))
	chained := make(map[string]*common.Load, len(order))
	for _, l := range order {
		stages[l.Name] = loadStagePending
		chained[l.Name] = l
	}

	for _, state := range client.GetLoadsStates(chained) {
		if state.Err == nil && state.Status.StatusCode == common.DeploymentStatusRunning {
			stages[state.Load.Name] = loadStageRunning
		}
	}

	for _, l := range order {
		if stages[l.Name] == loadStageRunning {
			log.Info("Starting %s skipped: load already running", color.CyanString(l.Name))
			continue
		}
		if err := client.StartLoad(l); err != nil {
			stages[l.Name] = loadStageStartFailed
			log.Warn("Starting %s failed: %s", color.CyanString(l.Name), err.Error())
			return stages
		}
		stages[l.Name] = loadStageRunning
		log.Success("Starting %s completed", color.CyanString(l.Name))
	}

	return stages
}

type loadStopStage string

const (
	loadStageStopPending loadStopStage = "pending"
	loadStageStopped     loadStopStage = "stopped"
	loadStageStopFailed  loadStopStage = "stop failed"
)

func isSuccessLoadsStopSequence(stages map[string]loadStopStage) bool {
	for _, stage := range stages {
		if stage != loadStageStopped {
			return false
		}
	}
	return true
}

// stopLoadsSequence stops the given loads following their stop chains,
// Returns the last report stop stages, if all loads are loadStageStopped then it's success
func (client *Client) stopLoadsSequence(loads map[string]*common.Load) map[string]loadStopStage {
	return client.stopOrderedLoadsSequence(StopOrder(loads))
}

// stopOrderedLoadsSequence stops the given loads in the given order,
func (client *Client) stopOrderedLoadsSequence(order []*common.Load) map[string]loadStopStage {
	stages := make(map[string]loadStopStage, len(order))
	chained := make(map[string]*common.Load, len(order))
	for _, l := range order {
		stages[l.Name] = loadStageStopPending
		chained[l.Name] = l
	}

	for _, state := range client.GetLoadsStates(chained) {
		if state.Err != nil {
			stages[state.Load.Name] = loadStageStopped
			log.Info("Stopping %s skipped: node not reachable: %s", color.CyanString(state.Load.Name), state.Err.Error())
		} else if !state.Provisioned {
			// Nothing deployed yet, e.g. a load rolled out on a fresh node
			stages[state.Load.Name] = loadStageStopped
		} else if state.Status.StatusCode != common.DeploymentStatusRunning {
			stages[state.Load.Name] = loadStageStopped
			log.Info("Stopping %s skipped: load not running", color.CyanString(state.Load.Name))
		}
	}

	for _, l := range order {
		if stages[l.Name] == loadStageStopped {
			continue
		}
		if err := client.StopLoad(l); err != nil {
			stages[l.Name] = loadStageStopFailed
			log.Warn("Stopping %s failed: %s", color.CyanString(l.Name), err.Error())
			return stages
		}
		stages[l.Name] = loadStageStopped
		log.Success("Stopping %s completed", color.CyanString(l.Name))
	}

	return stages
}
