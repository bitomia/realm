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

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Interface with node config",
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
					log.Info(" Status: %s [%s]", node.Status.StatusCode, "no configuration loaded")
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

var loadNodeConfig = &cobra.Command{
	Use:                   "load [--all | node...] [--validate]",
	Short:                 "Load node configurations",
	Args:                  validateNodeArgs,
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, nodeNames []string) {
		nodes := cfg.GetNodes(nodeNames...)
		client := clientPkg.NewClient(cfg)

		validationSuccess := true
		log.Info("Validating node configurations:")
		for _, n := range nodes {
			if err := client.ValidateNodeConfig(n); err != nil {
				log.Error("Node '%s' config validatioin error: %s", n.Name, err.Error())
				validationSuccess = false
			} else {
				log.Info("Node '%s' config validated", n.Name)
			}
		}

		if !validationSuccess {
			log.Error("Validation failed. Load aborted.")
			return
		}
		if validate, _ := cmd.Flags().GetBool("validate"); validate {
			log.Info("ONly validation")
			return
		}

		log.Info("Loading node configurations:")
		for _, n := range nodes {
			if err := client.LoadNodeConfig(n); err != nil {
				log.Warn("Node '%s' config loading error: %s", n.Name, err.Error())
			} else {
				log.Info("Node '%s' config loaded", n.Name)
			}
		}
	},
}

var unloadNodeConfig = &cobra.Command{
	Use:                   "unload [--all | node...]",
	Short:                 "Unload node configurations",
	Args:                  validateNodeArgs,
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, nodeNames []string) {
		nodes := cfg.GetNodes(nodeNames...)
		client := clientPkg.NewClient(cfg)

		log.Info("Unloading node configurations:")
		for _, n := range nodes {
			if err := client.UnloadNodeConfig(n); err != nil {
				log.Warn("Node '%s' config unloading failed: %s", n.Name, err.Error())
			} else {
				log.Info("Node '%s' config unloaded", n.Name)
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
		client := clientPkg.NewClient(cfg)

		log.Info("Powering on nodes:")
		for _, n := range nodes {
			if err := client.PowerOnNode(n); err != nil {
				log.Warn("Power-on node '%s' failed: %s", n.Name, err.Error())
			} else {
				log.Info("Node '%s' power-on done", n.Name)
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
		client := clientPkg.NewClient(cfg)

		log.Info("Powering off nodes:")
		for _, n := range nodes {
			if err := client.PowerOffNode(n); err != nil {
				log.Warn("Power off node '%s' failed: %s", n.Name, err.Error())
			} else {
				log.Info("Node '%s' power-off done", n.Name)
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
		client := clientPkg.NewClient(cfg)

		log.Info("Restarting nodes:")
		for _, n := range nodes {
			if err := client.RestartNode(n, "", 0); err != nil {
				log.Warn("Restarting node '%s' failed: %s", n.Name, err.Error())
			} else {
				log.Info("Node '%s' restarted", n.Name)
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
		client := clientPkg.NewClient(cfg)

		log.Info("Shutting down nodes:")
		for _, n := range nodes {
			if err := client.ShutdownNode(n, "", 0); err != nil {
				log.Warn("Shutting down node '%s' failed: %s", n.Name, err.Error())
			} else {
				log.Info("Node '%s' shutting down", n.Name)
			}
		}
	},
}

func init() {
	// Node config commands
	loadNodeConfig.Flags().Bool("all", false, "All nodes")
	loadNodeConfig.Flags().Bool("validate", false, "Validate only")
	unloadNodeConfig.Flags().Bool("all", false, "All nodes")

	configCmd.AddCommand(loadNodeConfig)
	configCmd.AddCommand(unloadNodeConfig)

	// General commands
	nodeStates.Flags().Bool("json", false, "Output as JSON")
	powerOnNodes.Flags().Bool("all", false, "All nodes")
	powerOffNodes.Flags().Bool("all", false, "All nodes")
	shutdownNodes.Flags().Bool("all", false, "All nodes")
	restartNodes.Flags().Bool("all", false, "All nodes")

	hostCmd.AddCommand(nodeStates)
	hostCmd.AddCommand(configCmd)
	hostCmd.AddCommand(powerOnNodes)
	hostCmd.AddCommand(powerOffNodes)
	hostCmd.AddCommand(shutdownNodes)
	hostCmd.AddCommand(restartNodes)

	rootCmd.AddCommand(hostCmd)
}
