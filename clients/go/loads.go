package golang

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"

	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/common/dto"
)

// LoadResultHandler is called after an operation has been run on a load,
// with the error returned by the operation if any.
type LoadResultHandler func(load *common.Load, err error)

// LoadState is the deployment state of a configured load.
type LoadState struct {
	Load           *common.Load
	Err            error
	Provisioned    bool
	Status         common.DeploymentStatus
	ContainerNames []string
}

// LoadsDependency is an edge of the loads dependency graph.
type LoadsDependency struct {
	From string
	To   string
}

func (c *Client) ProvisionLoad(load *common.Load) error {
	_, _, err := c.doJSONRequest("POST", load.Node.Url, "/loads/provision", load, requestTimeout)
	return err
}

func (c *Client) DeprovisionLoad(load *common.Load) error {
	_, _, err := c.doRequest("POST", load.Node.Url, fmt.Sprintf("/loads/%s/deprovision", load.Name), nil, requestTimeout)
	return err
}

func (c *Client) StartLoad(load *common.Load) error {
	_, _, err := c.doRequest("POST", load.Node.Url, fmt.Sprintf("/loads/%s/start", load.Name), nil, requestTimeout)
	return err
}

func (c *Client) StopLoad(load *common.Load) error {
	_, _, err := c.doRequest("POST", load.Node.Url, fmt.Sprintf("/loads/%s/stop", load.Name), nil, requestTimeout)
	return err
}

func (c *Client) KillLoad(load *common.Load) error {
	_, _, err := c.doRequest("POST", load.Node.Url, fmt.Sprintf("/loads/%s/kill", load.Name), nil, requestTimeout)
	return err
}

func (c *Client) GetLoadsDeployments(nodeUrl string) (dto.LoadsDeployments, error) {
	body, _, err := c.doRequest("GET", nodeUrl, "/loads", nil, requestTimeout)
	if err != nil {
		return nil, err
	}

	var loadDeployments dto.LoadsDeployments
	if err := json.Unmarshal(body, &loadDeployments); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	return loadDeployments, nil
}

func (c *Client) streamLoadOutput(load *common.Load, output string, w io.Writer) error {
	resp, err := c.doStreamRequest("GET", load.Node.Url, fmt.Sprintf("/loads/%s/%s", load.Name, output), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if _, err := io.Copy(w, resp.Body); err != nil {
		return fmt.Errorf("failed to stream %s: %v", output, err)
	}
	return nil
}

// ProvisionLoads provisions the given loads, stopping at the first failure.
func (c *Client) ProvisionLoads(loads map[string]*common.Load, handle LoadResultHandler) error {
	if len(loads) == 0 {
		return errors.New("no loads")
	}
	for _, load := range loads {
		err := c.ProvisionLoad(load)
		handle(load, err)
		if err != nil {
			return err
		}
	}
	return nil
}

// DeprovisionLoads deprovisions the given loads.
func (c *Client) DeprovisionLoads(loads map[string]*common.Load, handle LoadResultHandler) {
	for _, load := range loads {
		handle(load, c.DeprovisionLoad(load))
	}
}

// StartLoads starts the given loads following their start chains.
func (c *Client) StartLoads(loads map[string]*common.Load, handle LoadResultHandler) {
	runLoads(startOrder(loads), c.StartLoad, handle)
}

// StopLoads stops the given loads following their stop chains.
func (c *Client) StopLoads(loads map[string]*common.Load, handle LoadResultHandler) {
	runLoads(stopOrder(loads), c.StopLoad, handle)
}

// KillLoads kills the given loads following their stop chains.
func (c *Client) KillLoads(loads map[string]*common.Load, handle LoadResultHandler) {
	runLoads(stopOrder(loads), c.KillLoad, handle)
}

func startOrder(loads map[string]*common.Load) []*common.Load {
	var order []*common.Load
	added := make(map[string]bool)
	for _, name := range slices.Sorted(maps.Keys(loads)) {
		for _, l := range loads[name].StartChain {
			if added[l.Name] {
				continue
			}
			added[l.Name] = true
			order = append(order, l)
		}
	}
	return order
}

func stopOrder(loads map[string]*common.Load) []*common.Load {
	order := startOrder(loads)
	slices.Reverse(order)
	return order
}

func runLoads(loads []*common.Load, operation func(*common.Load) error, handle LoadResultHandler) {
	for _, l := range loads {
		handle(l, operation(l))
	}
}

// GetLoadsStates returns the deployment state of the given loads.
func (c *Client) GetLoadsStates(loads map[string]*common.Load) []LoadState {
	type nodeDeployments struct {
		byLoad map[string]dto.LoadsDeployments
		err    error
	}
	nodes := make(map[string]nodeDeployments)

	states := make([]LoadState, 0, len(loads))
	for _, name := range slices.Sorted(maps.Keys(loads)) {
		load := loads[name]

		node, exists := nodes[load.Node.Url]
		if !exists {
			deployments, err := c.GetLoadsDeployments(load.Node.Url)
			node = nodeDeployments{byLoad: make(map[string]dto.LoadsDeployments), err: err}
			for _, d := range deployments {
				node.byLoad[d.LoadName] = append(node.byLoad[d.LoadName], d)
			}
			nodes[load.Node.Url] = node
		}

		state := LoadState{Load: load, Err: node.err}
		if deployments, exists := node.byLoad[load.Name]; exists {
			state.Provisioned = true
			for _, d := range deployments {
				if name := containerName(d); name != "" {
					state.ContainerNames = append(state.ContainerNames, name)
				}
				state.Status = d.DeploymentStatus
			}
		}
		states = append(states, state)
	}
	return states
}

// containerName extracts the container name from the metadata of a container deployment.
func containerName(d dto.LoadDeployment) string {
	if d.Driver != "container" || d.Metadata == nil {
		return ""
	}
	metadataBytes, err := json.Marshal(d.Metadata)
	if err != nil {
		return ""
	}
	var metadata struct {
		ContainerName string `json:"container_name"`
	}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		return ""
	}
	return metadata.ContainerName
}

// LoadsDependencies returns the edges of the loads dependency graph.
func (c *Client) LoadsDependencies() ([]LoadsDependency, error) {
	adjacencyMap, err := c.cfg.GetLoadsGraph().AdjacencyMap()
	if err != nil {
		return nil, err
	}

	var dependencies []LoadsDependency
	for from, adjacencies := range adjacencyMap {
		if len(adjacencies) == 0 {
			dependencies = append(dependencies, LoadsDependency{From: from})
			continue
		}
		for to := range adjacencies {
			dependencies = append(dependencies, LoadsDependency{From: from, To: to})
		}
	}
	return dependencies, nil
}

func (c *Client) StreamLoadStdout(load *common.Load, w io.Writer) error {
	return c.streamLoadOutput(load, "stdout", w)
}

func (c *Client) StreamLoadStderr(load *common.Load, w io.Writer) error {
	return c.streamLoadOutput(load, "stderr", w)
}
