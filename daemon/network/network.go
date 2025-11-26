package network

import (
	"context"
	"crypto/sha1"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containernetworking/cni/libcni"
	types100 "github.com/containernetworking/cni/pkg/types/100"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/vishvananda/netlink"

	"github.com/bitomia/realm/daemon/cruntime"
	"github.com/bitomia/realm/daemon/db"
	"github.com/bitomia/realm/daemon/dns"
	"github.com/bitomia/realm/internal/config"
	"github.com/bitomia/realm/internal/dto"
)

const MIN_SUBNET = 167772160 // 10.0.0.0
const MAX_SUBNET = 184549120 // 10.255.255.0
const SUBNET_OFFSET = 256

type IPAddresses struct {
	Addresses []IP `json:"ips"`
}

func int32ToIPv4(i int32) net.IP {
	ipBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(ipBytes, uint32(i))

	return net.IP(ipBytes)
}

func getSubnet(network string) string {
	subnetOffset, err := db.GetDB().NewOrRetrieveSubnetOffset(network)
	if err != nil {
		slog.Error("Error acquiring new subnet", "error", err)
	}

	subnet := MIN_SUBNET + subnetOffset*SUBNET_OFFSET
	if subnet > MAX_SUBNET {
		slog.Error("No more subnets available!")
		os.Exit(1)
	}

	subnetAddr := int32ToIPv4(subnet)
	return fmt.Sprintf("%s/24", subnetAddr.String())
}

func createNetworkConfig(network string, subnet string, ipMask bool, portmaps []dto.Portmap) string {
	var ipMaskStr string
	if ipMask {
		ipMaskStr = "true"
	} else {
		ipMaskStr = "false"
	}
	portmappings := ""
	for _, portmap := range portmaps {
		portmappings += fmt.Sprintf(`{ "hostPort": %d, "containerPort": %d, "protocol": "%s"}`, portmap.HostPort, portmap.ContainerPort, portmap.Protocol)
	}
	portmapConfig := ""
	if len(portmappings) > 0 {
		portmapConfig = fmt.Sprintf(`, { "type": "portmap", "capabilities": { "portMappings": true }, "runtimeConfig": { "portMappings": [ %s ] } }`, portmappings)
	}

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
                    "10.0.1.1",
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
`, network, network, ipMaskStr, subnet, portmapConfig)
}

func generateInterfaceName(containerID string) string {
	// Hash the containerID to create a unique name and truncate to a reasonable length (e.g., 7 characters)
	hash := sha1.New()
	hash.Write([]byte(containerID))
	hashedID := fmt.Sprintf("%x", hash.Sum(nil))[:7]
	return "meth" + hashedID
}

func DeleteNetworkConfig(ctx context.Context, containerName string, pid uint32) error {
	cniPath := config.Get().Daemon.CniPath
	// TODO check cniPath exists
	cniConfig := libcni.NewCNIConfig(filepath.SplitList(cniPath), nil)

	db := db.GetDB()
	configs, _ := db.GetNetConfigs(containerName)
	for _, c := range configs {
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
		// TODO check if there is a better way without adding an extra dependency as netlink package
		isHostIfaceUsed, err := db.IsHostIfaceUsedExceptForContainer(c.HostIfaceName, containerName)
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
	db.DeleteAllNetConfigs(containerName)
	return nil
}

func StartNetwork(containerName string, opts dto.StartNetworkRequest) (error, map[string][]interface{}, net.IP, net.IP) {
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

	subnet := getSubnet(opts.Network)
	network := strings.Split(opts.Network, "-")[0]
	netConf := createNetworkConfig(network, subnet, opts.IPMasq, opts.PortMap)
	confList, err := libcni.ConfListFromBytes([]byte(netConf))
	if err != nil {
		slog.Error("Failed to parse CNI config", "error", err)
		os.Exit(1)
	}

	var gw net.IP
	var ip net.IPNet
	var networkConfigs = make(map[string][]interface{})
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
		// TODO check cniPath exists
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

		err = db.GetDB().AddNetConfig(opts.Network, containerName, []byte(netConf), []byte(resultJSON), ifaceName, opts.Network)
		if err != nil {
			return fmt.Errorf("Failed to marshal result: %s", err.Error()), nil, nil, nil
		}

		if opts.DNS {
			if gw != nil {
				slog.Info("Updating resolv.conf for container", "container", containerName, "pid", pid, "gw", gw)
				resolvConfContent := fmt.Sprintf("nameserver %s\nnameserver 8.8.8.8\nnameserver 8.8.4.4\n", gw)
				if err := WriteStringToResolvConf(ctx, task, resolvConfContent); err != nil {
					slog.Error("Error writing resolv.conf", "container", containerName, "error", err.Error())
					os.Exit(1)
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

	// Now create and execute a new write-resolv process
	execProcess, err := task.Exec(ctx, "write-resolv", &specs.Process{
		Args: []string{"/bin/sh", "-c", "echo '" + content + "' > /etc/resolv.conf"},
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

	if err := DeleteNetworkConfig(ctx, containerName, pid); err != nil {
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
		// TODO
	}
	for _, c := range configs {
		confList, err := libcni.ConfListFromBytes([]byte(c.Config))
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
	db := db.GetDB()

	var networks []IP
	configs, _ := db.GetNetConfigs(container)
	for _, config := range configs {
		var result IPAddresses
		json.Unmarshal([]byte(config.CniResult), &result)
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
func GetBridgeVethLinks() map[string]*Bridge {
	var bridges = make(map[string]*Bridge)

	links, err := netlink.LinkList()
	if err != nil {
		slog.Error("failed to list links", "error", err)
		os.Exit(1)
		return nil
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
	return bridges
}

func PurgeBridgeNetwork(bridge *Bridge) error {
	for _, l := range bridge.VethLinks {
		netlink.LinkDel(l)
	}
	if bridge == nil {
		return errors.New("bridge nil")
	}
	return netlink.LinkDel(bridge.Link)
}
