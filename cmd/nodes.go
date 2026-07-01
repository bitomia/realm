package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	clientPkg "github.com/bitomia/realm/cmd/client"
	"github.com/bitomia/realm/cmd/log"
	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/common/dto"
	"github.com/bitomia/realm/internal"
)

func validateNodeArgs(cmd *cobra.Command, args []string) error {
	all, _ := cmd.Flags().GetBool("all")

	if all && len(args) > 0 {
		return fmt.Errorf("cannot use --all with node names")
	}

	if !all && len(args) == 0 {
		return fmt.Errorf("must specify --all or at least one node name")
	}

	return nil
}

var hostCmd = &cobra.Command{
	Use:     "nodes",
	Aliases: []string{"n"},
	Short:   "Interface with nodes",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Realm CLI. Use -h for help.")
	},
}

var nodeStates = &cobra.Command{
	Use:                   "state",
	Short:                 "Show state of nodes",
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, nodeNames []string) {
		client := clientPkg.NewClient(cfg)
		nodes := cfg.GetNodes(nodeNames...)

		if asJson, _ := cmd.Flags().GetBool("json"); asJson {
			result := make(map[string]any, len(nodes))
			for id, cfgNode := range nodes {
				node, _ := client.GetNode(cfgNode)
				result[id] = struct {
					common.Node      `json:"config"`
					dto.NodeResponse `json:"node"`
				}{
					*cfgNode,
					node,
				}
			}
			json, _ := json.Marshal(result)
			fmt.Println(string(json))
		} else {
			for id, cfgNode := range nodes {
				fmt.Printf("Node: %s\n", color.CyanString(id))
				fmt.Printf(" URL: %s\n", color.CyanString(cfgNode.Url))
				node, err := client.GetNode(cfgNode)

				if node.Status.StatusCode == common.NodeStatusOnline {
					log.Info(" Status: %s [%s]", node.Status.StatusCode, "ready for provisioning")
				} else if err != nil {
					log.Info(" Status: %s [%s]", node.Status.StatusCode, strings.TrimSpace(err.Error()))
				} else {
					if len(node.Status.Reason) > 0 {
						log.Info(" Status: %s [%s]", node.Status.StatusCode, node.Status.Reason)
					} else {
						log.Info(" Status: %s", node.Status.StatusCode)
					}
					log.Info(" CPUs count: %s", color.CyanString(fmt.Sprintf("%d", node.State.NumCPU)))
					log.Info(" CPU Usage: %s%%", color.CyanString(fmt.Sprintf("%.2f", node.State.UsageCPUPercent)))
					log.Info(" User CPU: %s", color.CyanString(fmt.Sprintf("%d", node.State.UserCPU)))
					log.Info(" System CPU: %s", color.CyanString(fmt.Sprintf("%d", node.State.SystemCPU)))
					log.Info(" Idle CPU: %s", color.CyanString(fmt.Sprintf("%d", node.State.IdleCPU)))
					log.Info(" Total CPU: %s", color.CyanString(fmt.Sprintf("%d", node.State.TotalCPU)))

					log.Info(" Memory Usage: %s%%", color.CyanString(fmt.Sprintf("%.2f", float64(node.State.UsedMem)/float64(node.State.TotalMem)*100)))
					log.Info(" Free Memory: %s%%", color.CyanString(fmt.Sprintf("%.2f", node.State.FreeMemPercent)))
					log.Info(" Total Memory: %s MB", color.CyanString(fmt.Sprintf("%.2f", internal.ToMB(float64(node.State.TotalMem)))))
					log.Info(" Used Memory: %s MB", color.CyanString(fmt.Sprintf("%.2f", internal.ToMB(float64(node.State.UsedMem)))))
					log.Info(" Free Memory: %s MB", color.CyanString(fmt.Sprintf("%.2f", internal.ToMB(float64(node.State.FreeMem)))))
					log.Info(" Free Storage: %s MB", color.CyanString(fmt.Sprintf("%.2f", internal.ToMB(float64(node.State.FreeStorage)))))

					if len(node.State.Containers) > 0 {
						log.Info(" Containers (%d):", len(node.State.Containers))
						for _, container := range node.State.Containers {
							log.Info("  - %s:", color.YellowString(container.ContainerID))
							log.Info("    CPU Usage: %s%%", color.CyanString(fmt.Sprintf("%.2f", container.CPUUsage)))
							log.Info("    Memory Usage: %s%% (%s MB)",
								color.CyanString(fmt.Sprintf("%.2f", container.MemoryPercent)),
								color.CyanString(fmt.Sprintf("%.2f", internal.ToMB(container.MemoryUsage))))
							log.Info("    Memory Limit: %s MB", color.CyanString(fmt.Sprintf("%.2f", internal.ToMB(container.MemoryLimit))))
						}
					} else {
						log.Info(" Containers: %s", color.CyanString("0"))
					}

					if len(node.State.NetworkInterfaces) > 0 {
						log.Info(" Network Interfaces (%d):", len(node.State.NetworkInterfaces))
						for _, ni := range node.State.NetworkInterfaces {
							log.Info("  - %s (%s): %s",
								color.YellowString(ni.Name),
								color.CyanString(ni.HWAddr),
								color.CyanString(strings.Join(ni.Addresses, ", ")))
						}
					}
				}

				if node.Status.StatusCode != common.NodeStatusOffline {
					info, err := client.GetSystemInfo(cfgNode.Url)
					if err != nil {
						log.Info(" System Info: %s", strings.TrimSpace(err.Error()))
					} else {
						log.Info(" Capabilities:")
						log.Info("  Containers Engine: %s", color.CyanString(fmt.Sprintf("%t", info.Capabilities.ContainersEngine)))
						log.Info("  Containers Networking: %s", color.CyanString(fmt.Sprintf("%t", info.Capabilities.ContainersNetworking)))
						log.Info("  Volumes: %s", color.CyanString(fmt.Sprintf("%t", info.Capabilities.Volumes)))
						log.Info("  Volumes ZFS: %s", color.CyanString(fmt.Sprintf("%t", info.Capabilities.VolumesZFS)))
						log.Info("  VMM: %s", color.CyanString(fmt.Sprintf("%t", info.Capabilities.VMM)))
					}
				}
				fmt.Println()
			}
		}
	},
}

var registerNodes = &cobra.Command{
	Use:                   "register [--all | node...]",
	Short:                 "Register nodes on the cluster",
	Args:                  validateNodeArgs,
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, nodeNames []string) {
		nodes := cfg.GetNodes(nodeNames...)
		client := clientPkg.NewClient(cfg)

		for _, n := range nodes {
			log.Info("Registering node %s", color.CyanString(n.Name))
			if err := client.RegisterNode(n); err != nil {
				log.Fatal("Register node '%s' failed: %s", n.Name, err.Error())
			}
		}
	},
}

var unregisterNodes = &cobra.Command{
	Use:                   "unregister [--all | node...]",
	Short:                 "Unregister nodes from the cluster",
	Args:                  validateNodeArgs,
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, nodeNames []string) {
		nodes := cfg.GetNodes(nodeNames...)
		client := clientPkg.NewClient(cfg)

		for _, n := range nodes {
			log.Info("Unregistering node %s", color.CyanString(n.Name))
			if err := client.UnregisterNode(n); err != nil {
				log.Fatal("Unregister node '%s' failed: %s", n.Name, err.Error())
			}
		}
	},
}

var powerOnNodes = &cobra.Command{
	Use:                   "poweron [--all | node...]",
	Short:                 "Power-on nodes",
	Args:                  validateNodeArgs,
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, nodeNames []string) {
		nodes := cfg.GetNodes(nodeNames...)

		for _, n := range nodes {
			log.Info("Powering on node %s", color.CyanString(n.Name))
			driverInfo, err := n.Driver.Info()
			if err != nil {
				log.Fatal("Driver info for node '%s' failed: %s", n.Name, err.Error())
			}

			if driverInfo.PowerOnMode == common.ClientMode {
				if err := n.Driver.PowerOn(nil); err != nil {
					log.Fatal("Power on node '%s' failed: %s", n.Name, err.Error())
				}
			} else {
				client := clientPkg.NewClient(cfg)
				if err := client.PowerOnNode(n); err != nil {
					log.Fatal("Power on node '%s' failed: %s", n.Name, err.Error())
				}
			}
		}
	},
}

var powerOffNodes = &cobra.Command{
	Use:                   "poweroff [--all | node...]",
	Short:                 "Power-off nodes",
	Args:                  validateNodeArgs,
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, nodeNames []string) {
		nodes := cfg.GetNodes(nodeNames...)

		for _, n := range nodes {
			log.Info("Powering off node %s", color.CyanString(n.Name))
			driverInfo, err := n.Driver.Info()
			if err != nil {
				log.Fatal("Driver info for node '%s' failed: %s", n.Name, err.Error())
			}

			if driverInfo.PowerOnMode == common.ClientMode {
				if err := n.Driver.PowerOff(nil); err != nil {
					log.Fatal("Power off node '%s' failed: %s", n.Name, err.Error())
				}
			} else {
				client := clientPkg.NewClient(cfg)
				if err := client.PowerOffNode(n); err != nil {
					log.Fatal("Power off node '%s' failed: %s", n.Name, err.Error())
				}
			}
		}
	},
}

var restartNodes = &cobra.Command{
	Use:                   "restart [--all | node...]",
	Short:                 "Restart nodes",
	Args:                  validateNodeArgs,
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, nodeNames []string) {
		nodes := cfg.GetNodes(nodeNames...)

		for _, n := range nodes {
			driverInfo, err := n.Driver.Info()
			if err != nil {
				log.Fatal("Driver info for node '%s' failed: %s", n.Name, err.Error())
			}

			log.Info("Restarting node %s", color.CyanString(n.Name))
			if driverInfo.RestartMode == common.ClientMode {
				if err := n.Driver.Restart(nil, "", 0); err != nil {
					log.Fatal("Shutting down node '%s' failed: %s", n.Name, err.Error())
				}
			} else {
				client := clientPkg.NewClient(cfg)
				if err := client.RestartNode(n, "", 0); err != nil {
					log.Fatal("Restarting node '%s' failed: %s", n.Name, err.Error())
				}
			}
		}
	},
}

var shutdownNodes = &cobra.Command{
	Use:                   "shutdown [--all | node...]",
	Short:                 "Shutdown nodes",
	Args:                  validateNodeArgs,
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, nodeNames []string) {
		nodes := cfg.GetNodes(nodeNames...)

		for _, n := range nodes {
			driverInfo, err := n.Driver.Info()
			if err != nil {
				log.Fatal("Driver info for node '%s' failed: %s", n.Name, err.Error())
			}

			log.Info("Shutting down node %s", color.CyanString(n.Name))
			if driverInfo.ShutdownMode == common.ClientMode {
				if err := n.Driver.Shutdown(nil, "", 0); err != nil {
					log.Fatal("Shutting down node '%s' failed: %s", n.Name, err.Error())
				}
			} else {
				client := clientPkg.NewClient(cfg)
				if err := client.ShutdownNode(n, "", 0); err != nil {
					log.Fatal("Shutting down node '%s' failed: %s", n.Name, err.Error())
				}
			}
		}
	},
}

func init() {
	nodeStates.Flags().Bool("json", false, "Output as JSON")
	registerNodes.Flags().Bool("all", false, "All nodes")
	unregisterNodes.Flags().Bool("all", false, "All nodes")
	powerOnNodes.Flags().Bool("all", false, "All nodes")
	powerOffNodes.Flags().Bool("all", false, "All nodes")
	shutdownNodes.Flags().Bool("all", false, "All nodes")
	restartNodes.Flags().Bool("all", false, "All nodes")

	hostCmd.AddCommand(nodeStates)
	hostCmd.AddCommand(registerNodes)
	hostCmd.AddCommand(unregisterNodes)
	hostCmd.AddCommand(powerOnNodes)
	hostCmd.AddCommand(powerOffNodes)
	hostCmd.AddCommand(shutdownNodes)
	hostCmd.AddCommand(restartNodes)
	rootCmd.AddCommand(hostCmd)
}
