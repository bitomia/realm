package main

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	clientPkg "github.com/bitomia/realm/cmd/client"
	"github.com/bitomia/realm/cmd/log"
	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/internal"
)

func validateNodeArgs(cmd *cobra.Command, args []string) error {
	all, _ := cmd.Flags().GetBool("all")

	if all && len(args) > 0 {
		return fmt.Errorf("Cannot use --all with node names")
	}

	if !all && len(args) == 0 {
		return fmt.Errorf("Must specify --all or at least one node name")
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
					log.Info("Containers (%d):", len(node.State.Containers))
					for _, container := range node.State.Containers {
						log.Info("  - %s:", color.YellowString(container.ContainerID))
						log.Info("    CPU Usage: %s%%", color.CyanString(fmt.Sprintf("%.2f", container.CPUUsage)))
						log.Info("    Memory Usage: %s%% (%s MB)",
							color.CyanString(fmt.Sprintf("%.2f", container.MemoryPercent)),
							color.CyanString(fmt.Sprintf("%.2f", internal.ToMB(container.MemoryUsage))))
						log.Info("    Memory Limit: %s MB", color.CyanString(fmt.Sprintf("%.2f", internal.ToMB(container.MemoryLimit))))
					}
				} else {
					log.Info("Containers: %s", color.CyanString("0"))
				}

				info, err := client.GetSystemInfo(cfgNode.Url)
				if err != nil {
					log.Info("System Info: %s", strings.TrimSpace(err.Error()))
				} else {
					log.Info("Capabilities:")
					log.Info("  Containers Engine: %s", color.CyanString(fmt.Sprintf("%t", info.Capabilities.ContainersEngine)))
					log.Info("  Containers Networking: %s", color.CyanString(fmt.Sprintf("%t", info.Capabilities.ContainersNetworking)))
					log.Info("  Volumes: %s", color.CyanString(fmt.Sprintf("%t", info.Capabilities.Volumes)))
					log.Info("  Volumes ZFS: %s", color.CyanString(fmt.Sprintf("%t", info.Capabilities.VolumesZFS)))
					log.Info("  VMM: %s", color.CyanString(fmt.Sprintf("%t", info.Capabilities.VMM)))
				}

				if len(node.State.NetworkInterfaces) > 0 {
					log.Info("Network Interfaces (%d):", len(node.State.NetworkInterfaces))
					for _, ni := range node.State.NetworkInterfaces {
						log.Info("  - %s (%s): %s",
							color.YellowString(ni.Name),
							color.CyanString(ni.HWAddr),
							color.CyanString(strings.Join(ni.Addresses, ", ")))
					}
				}

				fmt.Println()
			}
		}
	},
}

var provisionNodes = &cobra.Command{
	Use:                   "provision [--all | node...]",
	Short:                 "Provision nodes on the cluster",
	Args:                  validateNodeArgs,
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, nodeNames []string) {
		nodes := cfg.GetNodes(nodeNames...)
		client := clientPkg.NewClient(cfg)

		for _, n := range nodes {
			log.Info("Provisioning node %s", color.CyanString(n.Name))
			if err := client.ProvisionNode(n); err != nil {
				log.Fatal("Provisioning node '%s' failed: %s", n.Name, err.Error())
			}
		}
	},
}

var deprovisionNodes = &cobra.Command{
	Use:                   "deprovision [--all | node...]",
	Short:                 "Deprovision nodes from the cluster",
	Args:                  validateNodeArgs,
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, nodeNames []string) {
		nodes := cfg.GetNodes(nodeNames...)
		client := clientPkg.NewClient(cfg)

		for _, n := range nodes {
			log.Info("Deprovisioning node %s", color.CyanString(n.Name))
			if err := client.DeprovisionNode(n); err != nil {
				log.Fatal("Deprovisioning node '%s' failed: %s", n.Name, err.Error())
			}
		}
	},
}

var startNodes = &cobra.Command{
	Use:                   "start [--all | node...]",
	Short:                 "Start nodes",
	Args:                  validateNodeArgs,
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, nodeNames []string) {
		nodes := cfg.GetNodes(nodeNames...)

		for _, n := range nodes {
			log.Info("Starting up node %s", color.CyanString(n.Name))
			driverInfo, err := n.Driver.DriverInfo()
			if err != nil {
				log.Fatal("Driver info for node '%s' failed: %s", n.Name, err.Error())
			}

			if driverInfo.StartMode == common.ClientMode {
				if err := n.Driver.Start(nil, nil); err != nil {
					log.Fatal("Starting node '%s' failed: %s", n.Name, err.Error())
				}
			} else {
				client := clientPkg.NewClient(cfg)
				if err := client.StartNode(n); err != nil {
					log.Fatal("Starting node '%s' failed: %s", n.Name, err.Error())
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
			driverInfo, err := n.Driver.DriverInfo()
			if err != nil {
				log.Fatal("Driver info for node '%s' failed: %s", n.Name, err.Error())
			}

			log.Info("Restarting node %s", color.CyanString(n.Name))
			if driverInfo.RestartMode == common.ClientMode {
				if err := n.Driver.Restart(nil, "", 0, nil); err != nil {
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

var stopNodes = &cobra.Command{
	Use:                   "stop [--all | node...] [--force]",
	Short:                 "Stop nodes",
	Args:                  validateNodeArgs,
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, nodeNames []string) {
		nodes := cfg.GetNodes(nodeNames...)
		force, _ := cmd.Flags().GetBool("force")

		for _, n := range nodes {
			driverInfo, err := n.Driver.DriverInfo()
			if err != nil {
				log.Fatal("Driver info for node '%s' failed: %s", n.Name, err.Error())
			}

			log.Info("Stopping node %s", color.CyanString(n.Name))
			if driverInfo.StopMode == common.ClientMode {
				if err := n.Driver.Stop(nil, "", 0, nil, force); err != nil {
					log.Fatal("Stopping node '%s' failed: %s", n.Name, err.Error())
				}
			} else {
				client := clientPkg.NewClient(cfg)
				if err := client.StopNode(n, "", 0, force); err != nil {
					log.Fatal("Stopping node '%s' failed: %s", n.Name, err.Error())
				}
			}
		}
	},
}

func init() {
	provisionNodes.Flags().Bool("all", false, "All nodes")
	deprovisionNodes.Flags().Bool("all", false, "All nodes")
	startNodes.Flags().Bool("all", false, "All nodes")
	restartNodes.Flags().Bool("all", false, "All nodes")
	stopNodes.Flags().Bool("all", false, "All nodes")
	stopNodes.Flags().Bool("force", false, "Force stop")

	hostCmd.AddCommand(nodeStates)
	hostCmd.AddCommand(provisionNodes)
	hostCmd.AddCommand(deprovisionNodes)
	hostCmd.AddCommand(startNodes)
	hostCmd.AddCommand(restartNodes)
	hostCmd.AddCommand(stopNodes)
	rootCmd.AddCommand(hostCmd)
}
