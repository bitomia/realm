package cmd

import (
	"github.com/fatih/color"

	clientPkg "github.com/bitomia/realm/clients/go"
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

// startLoadsSequence starts the given loads following their start chains,
// skipping the ones already running and stopping at the first failure.
// Returns the last report start stages, if all loads are loadStageRunning then it's success
func startLoadsSequence(client *clientPkg.Client, loads map[string]*common.Load) map[string]loadStartStage {
	order := clientPkg.StartOrder(loads)
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
// skipping the ones not running and stopping at the first failure.
// Returns the last report stop stages, if all loads are loadStageStopped then it's success
func stopLoadsSequence(client *clientPkg.Client, loads map[string]*common.Load) map[string]loadStopStage {
	order := clientPkg.StopOrder(loads)
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
