package golang

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net"
	"net/http"
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
	MeshIP  net.IP `json:"mesh_ip,omitempty"`
	Version string `json:"version,omitempty"`
}

func (c *Client) GetNode(node *common.Node) (dto.NodeResponse, error) {
	nodeRes := dto.NewNodeResponse()

	driverInfo, err := node.Driver.Info()
	if err != nil {
		return nodeRes, err
	}

	path := "/node"
	if driverInfo.GuestMode {
		path = guestNodePath(node.Name, "")
	}

	body, statusCode, err := c.doRequest("GET", node.Url, path, nil, 50*time.Second)
	if statusCode != -1 {
		// Node is online by default if it replied (with success or error)
		nodeRes.Status.StatusCode = common.NodeStatusOnline
	}
	if err != nil {
		return nodeRes, err
	}

	// Now we unmarshall the node reply and change StatusCode accordingly
	if err := json.Unmarshal(body, &nodeRes); err != nil {
		return nodeRes, fmt.Errorf("failed to parse JSON: %v", err)
	}

	return nodeRes, nil
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

// DiscoverNodes returns the nodes available on the cluster.
func (c *Client) DiscoverNodes(ctx context.Context, timeout time.Duration) (map[string]*NodeListEntry, error) {
	nodes := c.cfg.GetNodes()

	var wg sync.WaitGroup
	entries := make(map[string]*NodeListEntry, len(nodes))
	for name, node := range nodes {
		entry := &NodeListEntry{Name: name, Url: node.Url, Config: true}
		entries[name] = entry
	}

	discovered, mdnsErr := common.DiscoverAgents(ctx, timeout)
	c.discovered, c.discoverCached = discovered, true
	wg.Wait()

	meshNetworkIPs := meshNetworks()
	for _, agent := range discovered {
		matched := false
		for _, entry := range entries {
			if matchesDiscoveredAgent(entry.Url, agent) {
				entry.MDNS = true
				entry.MeshIP = c.filterByMeshIP(agent.IPs, meshNetworkIPs)
				matched = true
				break
			}
		}
		if !matched {
			entries[agent.Instance] = &NodeListEntry{Name: agent.Instance, Url: agent.URL(), MDNS: true, MeshIP: c.filterByMeshIP(agent.IPs, meshNetworkIPs)}
		}
	}

	for _, entry := range entries {
		wg.Go(func() {
			entry.Version, _ = c.GetVersion(entry.Url)
		})
	}
	wg.Wait()

	return entries, mdnsErr
}

// meshNetworks returns the IPv4 networks of the local netplane interfaces.
func meshNetworks() []*net.IPNet {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var networks []*net.IPNet
	for _, iface := range ifaces {
		if !strings.HasPrefix(iface.Name, "netplane") {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok && ipNet.IP.To4() != nil {
				networks = append(networks, ipNet)
			}
		}
	}
	return networks
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

func (c *Client) GetNodeConfig(node *common.Node) (*common.NodeDriverConfig, error) {
	payload, statusCode, err := c.doRequest("GET", node.Url, "/node/config", nil, requestTimeout)
	if statusCode == http.StatusNoContent {
		return nil, common.ErrNodeNotConfigured
	} else if err != nil {
		return nil, err
	} else {
		var nodeDriverConfig common.NodeDriverConfig
		if err := json.Unmarshal(payload, &nodeDriverConfig); err != nil {
			return nil, err
		}
		return &nodeDriverConfig, nil
	}
}

func (c *Client) LoadNodeConfig(node *common.Node) error {
	_, statusCode, err := c.doJSONRequest("POST", node.Url, "/node/config", node, requestTimeout)
	if statusCode == http.StatusConflict {
		return common.ErrNodeAlreadyConfigured
	} else {
		return err
	}
}

func (c *Client) ValidateNodeConfig(node *common.Node) error {
	_, statusCode, err := c.doJSONRequest("POST", node.Url, "/node/config?validate=true", node, requestTimeout)
	if statusCode == http.StatusConflict {
		return common.ErrNodeAlreadyConfigured
	} else {
		return err
	}
}

func (c *Client) UnloadNodeConfig(node *common.Node) error {
	driverInfo, err := node.Driver.Info()
	if err != nil {
		return err
	}

	path := "/node/config"
	if driverInfo.GuestMode {
		path = guestNodePath(node.Name, "/config")
	}

	_, _, err = c.doRequest("DELETE", node.Url, path, nil, requestTimeout)
	return err
}

// guestNodePath returns the agent path for a guest node, optionally followed by suffix.
func guestNodePath(name string, suffix string) string {
	return "/node/guests/" + url.PathEscape(name) + suffix
}

// runNodeOperation executes the operation locally when the driver declares it
// in client mode, or through the node agent otherwise. remote receives the agent
// path for the operation, which targets the guest node routes for guest drivers.
func runNodeOperation(node *common.Node, operation string, mode func(common.NodeDriverInfo) common.RunMode, local func() error, remote func(path string) error) error {
	driverInfo, err := node.Driver.Info()
	if err != nil {
		return fmt.Errorf("driver info: %w", err)
	}
	if mode(driverInfo) == common.ClientMode {
		return local()
	}
	if driverInfo.GuestMode {
		return remote(guestNodePath(node.Name, "/"+operation))
	}
	return remote("/node/" + operation)
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

func (c *Client) PowerOnNode(node *common.Node) error {
	return runNodeOperation(node, "poweron",
		func(i common.NodeDriverInfo) common.RunMode { return i.PowerOnMode },
		func() error { return node.Driver.PowerOn(node.CloudInit) },
		func(path string) error {
			_, _, err := c.doJSONRequest("POST", node.Url, path, node, requestTimeout)
			return err
		},
	)
}

func (c *Client) PowerOffNode(node *common.Node) error {
	return runNodeOperation(node, "poweroff",
		func(i common.NodeDriverInfo) common.RunMode { return i.PowerOffMode },
		node.Driver.PowerOff,
		func(path string) error {
			_, _, err := c.doRequest("POST", node.Url, path, nil, requestTimeout)
			return err
		},
	)
}

func (c *Client) ShutdownNode(node *common.Node, wallMessage string, offsetTime uint32) error {
	return runNodeOperation(node, "shutdown",
		func(i common.NodeDriverInfo) common.RunMode { return i.ShutdownMode },
		func() error { return node.Driver.Shutdown(wallMessage, offsetTime) },
		func(path string) error {
			request := dto.ShutdownNodeRequest{WallMessage: wallMessage, Time: offsetTime}
			_, _, err := c.doJSONRequest("POST", node.Url, path, request, requestTimeout)
			return err
		},
	)
}

func (c *Client) RestartNode(node *common.Node, wallMessage string, offsetTime uint32) error {
	return runNodeOperation(node, "restart",
		func(i common.NodeDriverInfo) common.RunMode { return i.RestartMode },
		func() error { return node.Driver.Restart(wallMessage, offsetTime) },
		func(path string) error {
			request := dto.RestartNodeRequest{WallMessage: wallMessage, Time: offsetTime}
			_, _, err := c.doJSONRequest("POST", node.Url, path, request, requestTimeout)
			return err
		},
	)
}
