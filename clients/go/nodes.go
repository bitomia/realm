package golang

import (
	"context"
	"errors"
	"maps"
	"net"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/common/dto"
)

var ErrNodesValidation = errors.New("node config validation failed")

type NodeResultHandler func(node *common.Node, err error)

type NodeState struct {
	Node     *common.Node
	Response dto.NodeResponse
	Err      error
	// SystemInfo is nil when the node is offline or SystemInfoErr is set.
	SystemInfo    *dto.SystemInfo
	SystemInfoErr error
}

// NodeListEntry are clusterd nodes found on the configuration
// or discovered through mDNS or both.
type NodeListEntry struct {
	Name string `json:"name"`
	Url  string `json:"url"`
	// Config is true if node is present on the configuration.
	Config bool `json:"config"`
	// MDNS is true if the node was found through mDNS discovery.
	MDNS    bool   `json:"mdns"`
	Version string `json:"version,omitempty"`
}

// GetNodesStates returns the state and system info of the given nodes.
func (c *Client) GetNodesStates(nodes map[string]*common.Node) []NodeState {
	states := make([]NodeState, 0, len(nodes))
	for _, name := range slices.Sorted(maps.Keys(nodes)) {
		node := nodes[name]
		state := NodeState{Node: node}
		state.Response, state.Err = c.GetNode(node)
		if state.Response.Status.StatusCode != common.NodeStatusOffline {
			state.SystemInfo, state.SystemInfoErr = c.GetSystemInfo(node.Url)
		}
		states = append(states, state)
	}
	return states
}

// ListNodes returns the nodes available on the cluster.
func (c *Client) ListNodes(ctx context.Context, timeout time.Duration) ([]*NodeListEntry, error) {
	nodes := c.cfg.GetNodes()

	var wg sync.WaitGroup
	entries := make(map[string]*NodeListEntry, len(nodes))
	for name, node := range nodes {
		entry := &NodeListEntry{Name: name, Url: node.Url, Config: true}
		entries[name] = entry
		wg.Go(func() {
			entry.Version, _ = c.GetVersion(entry.Url)
		})
	}

	discovered, mdnsErr := common.DiscoverAgents(ctx, timeout)
	wg.Wait()

	var discoveredOnly []*NodeListEntry
	for _, agent := range discovered {
		matched := false
		for _, entry := range entries {
			if matchesDiscoveredAgent(entry.Url, agent) {
				entry.MDNS = true
				matched = true
			}
		}
		if !matched {
			entry := &NodeListEntry{Name: agent.Instance, Url: agent.URL(), MDNS: true}
			discoveredOnly = append(discoveredOnly, entry)
			wg.Go(func() {
				entry.Version, _ = c.GetVersion(entry.Url)
			})
		}
	}
	wg.Wait()

	result := make([]*NodeListEntry, 0, len(entries)+len(discoveredOnly))
	for _, name := range slices.Sorted(maps.Keys(entries)) {
		result = append(result, entries[name])
	}
	result = append(result, discoveredOnly...)

	return result, mdnsErr
}

// matchesDiscoveredAgent tells whether a node url points to an agent discovered by mDNS,
// either by its ".local" name or by one of its addresses.
func matchesDiscoveredAgent(nodeUrl string, agent common.DiscoveredAgent) bool {
	parsed, err := url.Parse(nodeUrl)
	if err != nil {
		return false
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if host == "" {
		return false
	}
	if host == agent.Host {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && slices.ContainsFunc(agent.IPs, ip.Equal)
}

// ValidateNodesConfig validates the config of all given nodes, returning
// ErrNodesValidation if any of them is invalid.
func (c *Client) ValidateNodesConfig(nodes map[string]*common.Node, handle NodeResultHandler) error {
	valid := true
	for _, node := range nodes {
		err := c.ValidateNodeConfig(node)
		handle(node, err)
		if err != nil {
			valid = false
		}
	}
	if !valid {
		return ErrNodesValidation
	}
	return nil
}

// LoadNodesConfig loads the config to the given nodes.
func (c *Client) LoadNodesConfig(nodes map[string]*common.Node, handle NodeResultHandler) {
	runOnNodes(nodes, c.LoadNodeConfig, handle)
}

// UnloadNodesConfig unloads the config from the given nodes.
func (c *Client) UnloadNodesConfig(nodes map[string]*common.Node, handle NodeResultHandler) {
	runOnNodes(nodes, c.UnloadNodeConfig, handle)
}

// PowerOnNodes powers on the given nodes.
func (c *Client) PowerOnNodes(nodes map[string]*common.Node, handle NodeResultHandler) {
	runOnNodes(nodes, c.PowerOnNode, handle)
}

// PowerOffNodes powers off the given nodes.
func (c *Client) PowerOffNodes(nodes map[string]*common.Node, handle NodeResultHandler) {
	runOnNodes(nodes, c.PowerOffNode, handle)
}

// ShutdownNodes shuts down the given nodes.
func (c *Client) ShutdownNodes(nodes map[string]*common.Node, wallMessage string, offsetTime uint32, handle NodeResultHandler) {
	runOnNodes(nodes, func(n *common.Node) error { return c.ShutdownNode(n, wallMessage, offsetTime) }, handle)
}

// RestartNodes restarts the given nodes.
func (c *Client) RestartNodes(nodes map[string]*common.Node, wallMessage string, offsetTime uint32, handle NodeResultHandler) {
	runOnNodes(nodes, func(n *common.Node) error { return c.RestartNode(n, wallMessage, offsetTime) }, handle)
}

func runOnNodes(nodes map[string]*common.Node, operation func(*common.Node) error, handle NodeResultHandler) {
	for _, node := range nodes {
		handle(node, operation(node))
	}
}
