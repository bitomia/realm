package network

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containernetworking/cni/libcni"
	types100 "github.com/containernetworking/cni/pkg/types/100"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/vishvananda/netlink"

	"github.com/bitomia/realm/common/config"
	"github.com/bitomia/realm/common/dto"
	"github.com/bitomia/realm/daemon/cruntime"
	"github.com/bitomia/realm/daemon/db"
	"github.com/bitomia/realm/daemon/dns"
)

const MIN_SUBNET = 167772160 // 10.0.0.0
const MAX_SUBNET = 184549120 // 10.255.255.0
const SUBNET_OFFSET = 256

// Required CNI plugins
var (
	requiredCNIPlugins    = []string{"bridge", "host-local", "firewall", "portmap"}
	winRequiredCNIPlugins = []string{"host-local.exe", "win-bridge.exe", "win-overlay.exe"}
)

type IPAddresses struct {
	Addresses []IP `json:"ips"`
}

func int32ToIPv4(i int32) net.IP {
	ipBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(ipBytes, uint32(i))

	return net.IP(ipBytes)
}

func getSubnet(network string) (string, error) {
	subnetOffset, err := db.GetDB().NewOrRetrieveSubnetOffset(network)
	if err != nil {
		return "", fmt.Errorf("error acquiring new subnet: %w", err)
	}

	subnet := MIN_SUBNET + subnetOffset*SUBNET_OFFSET
	if subnet > MAX_SUBNET {
		return "", errors.New("no more subnets available")
	}

	subnetAddr := int32ToIPv4(subnet)
	return fmt.Sprintf("%s/24", subnetAddr.String()), nil
}

// deriveGatewayFromSubnet extracts the gateway IP from a subnet CIDR (e.g., "10.5.0.0/24" -> "10.5.0.1")
func deriveGatewayFromSubnet(subnet string) string {
	_, ipNet, err := net.ParseCIDR(subnet)
	if err != nil {
		slog.Error("Failed to parse subnet CIDR, using fallback", "subnet", subnet, "error", err)
		return "10.0.1.1"
	}

	// Get the network address and add 1 to get the gateway
	ip := ipNet.IP.To4()
	if ip == nil {
		slog.Error("Subnet is not IPv4, using fallback", "subnet", subnet)
		return "10.0.1.1"
	}

	// Gateway is typically the first usable IP in the subnet (network address + 1)
	gateway := make(net.IP, 4)
	copy(gateway, ip)
	gateway[3]++

	return gateway.String()
}

func createNetworkConfig(network string, subnet string, ipMask bool, portmaps []dto.Portmap) string {
	var ipMaskStr string
	if ipMask {
		ipMaskStr = "true"
	} else {
		ipMaskStr = "false"
	}
	var portmapEntries []string
	for _, portmap := range portmaps {
		portmapEntries = append(portmapEntries, fmt.Sprintf(
			`{ "hostPort": %d, "containerPort": %d, "protocol": "%s"}`,
			portmap.HostPort, portmap.ContainerPort, portmap.Protocol))
	}
	portmappings := strings.Join(portmapEntries, ", ")
	portmapConfig := ""
	if len(portmappings) > 0 {
		portmapConfig = fmt.Sprintf(`, { "type": "portmap", "capabilities": { "portMappings": true }, "runtimeConfig": { "portMappings": [ %s ] } }`, portmappings)
	}

	// Derive the gateway IP from the subnet for DNS nameserver
	gatewayIP := deriveGatewayFromSubnet(subnet)

	return fmt.Sprintf(`
{
    "cniVersion": "1.0.0",
    "name": "%s",
    "plugins": [
        {
            "type": "bridge",
            "bridge": "%s",
            "isGateway": true,
            "ipMasq": %s,
            "ipam": {
                "type": "host-local",
                "subnet": "%s",
                "routes": [
                    { "dst": "0.0.0.0/5" },
                    { "dst": "8.0.0.0/7" },
                    { "dst": "11.0.0.0/8" },
                    { "dst": "12.0.0.0/6" },
                    { "dst": "16.0.0.0/4" },
                    { "dst": "32.0.0.0/3" },
                    { "dst": "64.0.0.0/2" },
                    { "dst": "128.0.0.0/1" }
                ]
            },
            "dns": {
                "nameservers": [
                    "%s",
                    "8.8.8.8",
                    "8.8.4.4"
                ]
            }
        },
        {
            "type": "firewall",
            "backend": "iptables"
        }%s
    ]
}
`, network, network, ipMaskStr, subnet, gatewayIP, portmapConfig)
}

func generateInterfaceName(containerID string) string {
	// Hash the containerID to create a unique name and truncate to a reasonable length (e.g., 7 characters)
	hash := sha1.New()
	hash.Write([]byte(containerID))
	hashedID := fmt.Sprintf("%x", hash.Sum(nil))[:7]
	return "meth" + hashedID
}

func getBridgeName(network string) string {
	return strings.Split(network, "-")[0]
}

func deleteBridge(bridgeName string) error {
	link, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return nil
	}
	if err := netlink.LinkDel(link); err != nil {
		return fmt.Errorf("failed to delete bridge %s: %w", bridgeName, err)
	}
	slog.Info("Bridge deleted", "bridge", bridgeName)
	return nil
}

func deleteNetworkConfig(ctx context.Context, containerName string, pid uint32) error {
	cniPath := config.Get().Daemon.CniPath
	// CNI path validated at daemon startup via ValidateCNIAvailability()
	cniConfig := libcni.NewCNIConfig(filepath.SplitList(cniPath), nil)

	dbConn := db.GetDB()
	configs, err := dbConn.GetNetConfigs(containerName)
	if err != nil {
		slog.Error("Failed to get network configs, continuing with cleanup", "container", containerName, "error", err)
	}

	// Track networks used by this container for subnet cleanup
	networksToCheck := make(map[string]bool)

	for _, c := range configs {
		networksToCheck[c.Network] = true

		confList, err := libcni.ConfListFromBytes([]byte(c.Config))
		if err != nil {
			return fmt.Errorf("Failed to parse CNI config: %s\n", err)
		}

		netns := fmt.Sprintf("/proc/%d/ns/net", pid)
		err = cniConfig.DelNetworkList(ctx, confList, &libcni.RuntimeConf{
			ContainerID: containerName,
			NetNS:       netns,
			IfName:      c.GuestIfaceName,
		})
		if err != nil {
			slog.Error("Failed to delete network. Continuing.", "error", err)
		}

		// we have to delete the host iface because CNI bridge plugin is not doing this
		isHostIfaceUsed, err := dbConn.IsHostIfaceUsedExceptForContainer(c.HostIfaceName, containerName)
		if err != nil {
			return errors.New("isHostIfaceUsed call failed")
		} else if !isHostIfaceUsed {
			slog.Info("Host interface not used anymore while deleting net config for container, cleaning it up", "container", containerName)
			link, err := netlink.LinkByName(c.HostIfaceName)
			if err != nil {
				slog.Info("Failed to get link for container. Continuing.", "container", containerName, "hostIface", c.HostIfaceName, "error", err)
			} else {
				err = netlink.LinkDel(link)
				if err != nil {
					slog.Error("Failed to delete link", "container", containerName, "netns", netns, "error", err)
				}
			}
		}
	}
	dbConn.DeleteAllNetConfigs(containerName)

	// Release subnet for networks that no longer have any containers
	for network := range networksToCheck {
		count, err := dbConn.GetNetworkContainerCount(network)
		if err != nil {
			slog.Error("Failed to get network container count", "network", network, "error", err)
			continue
		}
		if count == 0 {
			slog.Info("Network has no more containers, releasing subnet and deleting bridge", "network", network)
			if err := dbConn.ReleaseSubnet(network); err != nil {
				slog.Error("Failed to release subnet", "network", network, "error", err)
			}
			// Delete the bridge since no containers are using this network anymore
			bridgeName := getBridgeName(network)
			if err := deleteBridge(bridgeName); err != nil {
				slog.Error("Failed to delete bridge", "bridge", bridgeName, "error", err)
			}
		}
	}

	return nil
}

func StartNetwork(containerName string, netConfig dto.NetworkConfig) (error, map[string][]any, net.IP, net.IP) {
	ctx, client, err := cruntime.CreateClient()
	if err != nil {
		return fmt.Errorf("Cannot create cruntime client: %s - %s", containerName, err.Error()), nil, nil, nil
	}
	defer client.Close()

	containers, err := client.Containers(ctx)
	if err != nil {
		slog.Info("network.StartNetwork: Cannot retrieve containers client")
		return err, nil, nil, nil
	}

	subnet, err := getSubnet(netConfig.Network)
	if err != nil {
		return fmt.Errorf("failed to get subnet: %w", err), nil, nil, nil
	}
	bridgeName := getBridgeName(netConfig.Network)
	netConf := createNetworkConfig(bridgeName, subnet, netConfig.IPMasq, netConfig.PortMap)
	confList, err := libcni.ConfListFromBytes([]byte(netConf))
	if err != nil {
		return fmt.Errorf("failed to parse CNI config: %w", err), nil, nil, nil
	}

	var gw net.IP
	var ip net.IPNet
	var networkConfigs = make(map[string][]any)
	for _, container := range containers {
		if container.ID() != containerName {
			continue
		}

		task, err := container.Task(ctx, nil)
		if err != nil {
			return fmt.Errorf("Error while retrieving task for container %s: %s", containerName, err.Error()), nil, nil, nil
		}
		if task == nil {
			continue
		}

		pid := task.Pid()

		cniPath := config.Get().Daemon.CniPath
		// CNI path validated at daemon startup via ValidateCNIAvailability()
		cniConfig := libcni.NewCNIConfig(filepath.SplitList(cniPath), nil)

		netns := fmt.Sprintf("/proc/%d/ns/net", pid)
		containerID := containerName
		ifaceName := generateInterfaceName(containerName)

		result, err := cniConfig.AddNetworkList(ctx, confList, &libcni.RuntimeConf{
			ContainerID: containerID,
			NetNS:       netns,
			IfName:      ifaceName,
		})
		if err != nil {
			return fmt.Errorf("Failed to add network for container=%s pid=%d: %v\n", containerName, pid, err), nil, nil, nil
		}
		resultJSON, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("Failed to marshal result: %s", err.Error()), nil, nil, nil
		}

		networkConfigs[containerName] = append(networkConfigs[containerName], result)
		networkCNIConfig := networkConfigs[containerName][0].(*types100.Result)
		if len(networkCNIConfig.IPs) > 0 {
			gw = networkCNIConfig.IPs[0].Gateway
			ip = networkCNIConfig.IPs[0].Address
			if err := dns.RegisterContainerDNS(containerName, ip); err != nil {
				slog.Error("Error registering container in DNS", "container", containerName)
			}
		}

		err = db.GetDB().AddNetConfig(netConfig.Network, containerName, []byte(netConf), []byte(resultJSON), ifaceName, netConfig.Network)
		if err != nil {
			return fmt.Errorf("Failed to marshal result: %s", err.Error()), nil, nil, nil
		}

		if netConfig.DNS {
			if gw != nil {
				slog.Info("Updating resolv.conf for container", "container", containerName, "pid", pid, "gw", gw)
				resolvConfContent := fmt.Sprintf("nameserver %s\nnameserver 8.8.8.8\nnameserver 8.8.4.4\n", gw)
				if err := WriteStringToResolvConf(ctx, task, resolvConfContent); err != nil {
					return fmt.Errorf("error writing resolv.conf for container %s: %w", containerName, err), nil, nil, nil
				}
			} else {
				slog.Error("Cannot update resolv.conf for container", "container", containerName, "pid", pid)
			}
		}
	}

	return nil, networkConfigs, gw, ip.IP
}

func WriteStringToResolvConf(ctx context.Context, task containerd.Task, content string) error {
	// Try to clean-up any other write-resolv process
	oldExecProcess, err := task.LoadProcess(ctx, "write-resolv", cio.NewAttach(cio.WithStdio))
	if err == nil && oldExecProcess != nil {
		oldExecProcess.Delete(ctx)
	}

	// Use base64 encoding to safely pass content without shell injection
	encoded := base64.StdEncoding.EncodeToString([]byte(content))

	// Now create and execute a new write-resolv process
	execProcess, err := task.Exec(ctx, "write-resolv", &specs.Process{
		Args: []string{"/bin/sh", "-c", fmt.Sprintf("echo %s | base64 -d > /etc/resolv.conf", encoded)},
		Cwd:  "/",
	}, cio.NewCreator(cio.WithStdio))
	if err != nil {
		return err
	}
	defer execProcess.Delete(ctx)

	if err := execProcess.Start(ctx); err != nil {
		return err
	}

	// Wait for the exec process to finish
	_, err = execProcess.Wait(ctx)
	return err
}

func DeleteNetwork(containerName string) error {
	ctx, client, err := cruntime.CreateClient()
	if err != nil {
		return fmt.Errorf("Cannot create cruntime client: %s - %s", containerName, err.Error())
	}
	defer client.Close()

	pid, err := cruntime.GetContainerTaskPID(ctx, client, containerName)
	if err != nil {
		return err
	}
	dns.UnregisterContainerDNS(containerName)

	if err := deleteNetworkConfig(ctx, containerName, pid); err != nil {
		return err
	}
	return nil
}

func RepairNetwork(containerName string) error {
	ctx, client, err := cruntime.CreateClient()
	if err != nil {
		return fmt.Errorf("Cannot create cruntime client: %s - %s", containerName, err.Error())
	}
	defer client.Close()

	container, err := client.LoadContainer(ctx, containerName)
	if err != nil {
		return err
	}

	task, err := container.Task(ctx, nil)
	if err != nil {
		return fmt.Errorf("Error while retrieving task for container %s: %s", containerName, err.Error())
	}
	if task == nil {
		return errors.New("Cannot find task")
	}
	pid := task.Pid()
	cniPath := config.Get().Daemon.CniPath
	cniConfig := libcni.NewCNIConfig(filepath.SplitList(cniPath), nil)
	netns := fmt.Sprintf("/proc/%d/ns/net", pid)

	configs, err := db.GetDB().GetNetConfigs(containerName)
	if err != nil {
		return fmt.Errorf("failed to get network configs: %w", err)
	}
	for _, c := range configs {
		confList, err := libcni.ConfListFromBytes([]byte(c.Config))
		if err != nil {
			return fmt.Errorf("failed to parse CNI config: %w", err)
		}
		_, rt, err := cniConfig.GetNetworkListCachedConfig(confList, &libcni.RuntimeConf{
			ContainerID: containerName,
			NetNS:       netns,
			IfName:      c.GuestIfaceName,
		})
		if err != nil {
			return err
		}
		if rt == nil {
			return fmt.Errorf("RepairNetwork failed, runtime config is nil")
		}
		err = cniConfig.DelNetworkList(ctx, confList, rt)
		if err != nil {
			return err
		}
		result, err := cniConfig.AddNetworkList(ctx, confList, &libcni.RuntimeConf{
			ContainerID: containerName,
			NetNS:       netns,
			IfName:      c.GuestIfaceName,
		})
		if err != nil {
			return err
		}
		var networkConfigs = make(map[string][]interface{})
		networkConfigs[containerName] = append(networkConfigs[containerName], result)
		networkCNIConfig := networkConfigs[containerName][0].(*types100.Result)
		if len(networkCNIConfig.IPs) > 0 {
			ip := networkCNIConfig.IPs[0].Address
			if err := dns.RegisterContainerDNS(containerName, ip); err != nil {
				slog.Error("Error registering container in DNS", "container", containerName)
			}
		}
	}

	// if err != nil {
	// 	return errors.New(fmt.Sprintf("Failed to add network for container=%s pid=%d: %v\n", containerName, pid, err))
	// }

	// resultJSON, err := json.Marshal(result)
	// if err != nil {
	// 	return errors.New(fmt.Sprintf("Failed to marshal result: %s", err.Error()))
	// }

	return nil
}

type IP struct {
	Address string `json:"address"`
	Gateway string `json:"gateway"`
}

func GetNetworkConfig(container string) []IP {
	dbConn := db.GetDB()

	var networks []IP
	configs, err := dbConn.GetNetConfigs(container)
	if err != nil {
		slog.Error("Failed to get network configs", "container", container, "error", err)
		return networks
	}
	for _, config := range configs {
		var result IPAddresses
		if err := json.Unmarshal([]byte(config.CniResult), &result); err != nil {
			slog.Error("Failed to unmarshal CNI result", "container", container, "error", err)
			continue
		}
		for _, ip := range result.Addresses {
			networks = append(networks, ip)
		}
	}
	return networks
}

type Bridge struct {
	Link      netlink.Link
	VethLinks []netlink.Link
}

// Returns bridges with veth links from the host system
func GetBridgeVethLinks() (map[string]*Bridge, error) {
	var bridges = make(map[string]*Bridge)

	links, err := netlink.LinkList()
	if err != nil {
		return nil, fmt.Errorf("failed to list links: %w", err)
	}

	for _, link := range links {
		if link.Type() == "bridge" {
			bridgeName := link.Attrs().Name
			bridgeIndex := link.Attrs().Index
			bridges[bridgeName] = &Bridge{}
			bridges[bridgeName].Link = link
			for _, l := range links {
				if l.Attrs().MasterIndex == bridgeIndex && l.Type() == "veth" {
					bridges[bridgeName].VethLinks = append(bridges[bridgeName].VethLinks, l)
				}
			}
		}
	}
	return bridges, nil
}

func PurgeBridgeNetwork(bridge *Bridge) error {
	if bridge == nil {
		return errors.New("bridge nil")
	}
	for _, l := range bridge.VethLinks {
		netlink.LinkDel(l)
	}
	return netlink.LinkDel(bridge.Link)
}

// IsCNIAvailable checks if CNI path and all required plugins exist
func IsCNIAvailable() error {
	cniPath := config.Get().Daemon.CniPath

	// Check if CNI path exists
	if _, err := os.Stat(cniPath); os.IsNotExist(err) {
		return fmt.Errorf("CNI path does not exist: %s", cniPath)
	}

	var missingPlugins []string

	// Check each plugin
	var requiredCNIPlugins = requiredCNIPlugins
	if runtime.GOOS == "windows" {
		requiredCNIPlugins = winRequiredCNIPlugins
	}
	for _, plugin := range requiredCNIPlugins {
		pluginPath := filepath.Join(cniPath, plugin)
		if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
			missingPlugins = append(missingPlugins, plugin)
		}
	}

	if len(missingPlugins) > 0 {
		return fmt.Errorf("missing required CNI plugins in %s: %s",
			cniPath, strings.Join(missingPlugins, ", "))
	}

	return nil
}

type PurgedNetworkInfo struct {
	CNIPaths []string `json:"cni_paths"`
	Bridges  []string `json:"bridges"`
}

func PurgeNetworks() error {
	ctx, client, err := cruntime.CreateClient()
	if err != nil {
		return err
	}
	defer client.Close()

	containersList, err := client.ContainerService().List(ctx)
	if err != nil {
		return err
	}

	dbConn := db.GetDB()
	hostIfaces := []string{}
	for _, container := range containersList {
		configs, _ := dbConn.GetNetConfigs(container.ID)
		for _, config := range configs {
			hostIfaces = append(hostIfaces, config.HostIfaceName)
		}
	}

	result := PurgedNetworkInfo{}

	// Purge orphaned bridges and links
	bridges, err := GetBridgeVethLinks()
	if err != nil {
		return err
	}
	for bridgeName, bridge := range bridges {
		if !slices.Contains(hostIfaces, bridgeName) {
			slog.Info("Purging %s orphaned bridge network", bridgeName)
			if err := PurgeBridgeNetwork(bridge); err != nil {
				slog.Info("Ignoring puring bridge error: %s", err.Error())
			} else {
				result.Bridges = append(result.Bridges, bridge.Link.Attrs().Name)
			}
		}
	}

	// Purge orphaned CNI networks
	networkDir := "/var/lib/cni/networks"
	files, err := os.ReadDir(networkDir)
	if err != nil {
		return err
	}
	for _, file := range files {
		if !slices.Contains(hostIfaces, file.Name()) {
			path := filepath.Join(networkDir, file.Name())
			slog.Info("Purging %s orphaned CNI config: %s", file.Name(), path)
			os.RemoveAll(path)
			result.CNIPaths = append(result.CNIPaths, path)
		}
	}

	return nil
}
