package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"

	"github.com/bitomia/realm/common"
)

func (c *Config) WithLoad(name string, load common.LoadConfig) (*Config, error) {
	raw := *c
	raw.Loads = make(LoadsConfig, len(c.Loads)+1)
	maps.Copy(raw.Loads, c.Loads)
	raw.Loads[name] = load

	return c.rebuild(raw)
}

func (c *Config) WithNode(name string, node common.NodeConfig) (*Config, error) {
	raw := *c
	raw.Nodes = make(NodesConfig, len(c.Nodes)+1)
	maps.Copy(raw.Nodes, c.Nodes)
	raw.Nodes[name] = &node

	return c.rebuild(raw)
}

func (c *Config) WithoutLoad(name string) (*Config, error) {
	raw := *c
	raw.Loads = maps.Clone(c.Loads)
	delete(raw.Loads, name)

	return c.rebuild(raw)
}

func (c *Config) WithoutNode(name string) (*Config, error) {
	raw := *c
	raw.Nodes = maps.Clone(c.Nodes)
	delete(raw.Nodes, name)

	return c.rebuild(raw)
}

func (c *Config) WithJob(name string, job common.JobConfig) (*Config, error) {
	raw := *c
	raw.Jobs = make(JobsConfig, len(c.Jobs)+1)
	maps.Copy(raw.Jobs, c.Jobs)
	raw.Jobs[name] = job

	return c.rebuild(raw)
}

func (c *Config) WithoutJob(name string) (*Config, error) {
	raw := *c
	raw.Jobs = maps.Clone(c.Jobs)
	delete(raw.Jobs, name)

	return c.rebuild(raw)
}

func (c *Config) rebuild(raw Config) (newCfg *Config, err error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize config: %w", err)
	}

	prevNodes, prevJobs, prevLoads, prevGraph := nodesConfig, jobsConfig, loadsConfig, loadsConfigGraph
	defer func() {
		// readConfig panics on some invalid inputs, e.g. node expressions
		if r := recover(); r != nil {
			newCfg, err = nil, fmt.Errorf("invalid config: %v", r)
		}
		if err != nil {
			nodesConfig, jobsConfig, loadsConfig, loadsConfigGraph = prevNodes, prevJobs, prevLoads, prevGraph
		}
	}()

	ResetNodesConfig()
	ResetJobsConfig()
	ResetLoadsConfig()

	newCfg, err = readConfig(unmarshalConfigHandler, bytes.NewReader(data), "")
	if err != nil {
		return nil, err
	}
	newCfg.dataPathSet = c.dataPathSet

	return newCfg, nil
}
