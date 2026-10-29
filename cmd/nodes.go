package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	clientPkg "github.com/bitomia/realm/clients/go"
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
			for _, state := range client.GetNodesStates(nodes) {
				result[state.Node.Name] = struct {
					common.Node      `json:"config"`
					dto.NodeResponse `json:"node"`
				}{
					*state.Node,
					state.Response,
				}
			}
			json, _ := json.Marshal(result)
			fmt.Println(string(json))
		} else {
			for _, state := range client.GetNodesStates(nodes) {
				fmt.Printf("Node: %s\n", color.CyanString(state.Node.Name))
				fmt.Printf(" URL: %s\n", color.CyanString(state.Node.Url))
				node, err := state.Response, state.Err

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
					if info, err := state.SystemInfo, state.SystemInfoErr; err != nil {
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

var listNodes = &cobra.Command{
	Use:                   "discover",
	Short:                 "Discover configured nodes and agents through mDNS",
	Args:                  cobra.NoArgs,
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		timeout, _ := cmd.Flags().GetDuration("timeout")
		client := clientPkg.NewClient(cfg)
		result, mdnsErr := client.DiscoverNodes(cmd.Context(), timeout)
		if mdnsErr != nil {
			log.Warn("mDNS discovery failed: %s", mdnsErr.Error())
		}

		if asJson, _ := cmd.Flags().GetBool("json"); asJson {
			json, _ := json.Marshal(result)
			fmt.Println(string(json))
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tSOURCE\tURL\tIP\tVERSION")
		for _, entry := range result {
			var sources []string
			if entry.Config {
				sources = append(sources, "config")
			}
			if entry.MDNS {
				sources = append(sources, "mdns")
			}
			version := entry.Version
			if version == "" {
				version = "-"
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", entry.Name, strings.Join(sources, "+"), entry.Url, entry.MeshIP.String(), version)
		}
		_ = w.Flush()
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

		err := client.ValidateNodesConfig(nodes, func(n *common.Node, err error) {
			if err == nil {
				log.Success("Validating %s completed", color.CyanString(n.Name))
			} else {
				log.Warn("Validating %s failed: %s", color.CyanString(n.Name), err.Error())
			}
		})
		if err != nil {
			log.Error("Loading aborted: %s", err.Error())
			return
		}
		if validate, _ := cmd.Flags().GetBool("validate"); validate {
			return
		}

		client.LoadNodesConfig(nodes, func(n *common.Node, err error) {
			if err == nil {
				log.Success("Loading %s completed", color.CyanString(n.Name))
			} else {
				log.Warn("Loading %s failed: %s", color.CyanString(n.Name), err.Error())
			}
		})
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
		client.UnloadNodesConfig(nodes, func(n *common.Node, err error) {
			if err == nil {
				log.Success("Unloading %s completed", color.CyanString(n.Name))
			} else {
				log.Warn("Unloading %s failed: %s", color.CyanString(n.Name), err.Error())
			}
		})
	},
}

// nodeStartStage is how far a node got through the start sequence.
type nodeStartStage string

const (
	nodeStagePending    nodeStartStage = "pending"
	nodeStagePoweredOn  nodeStartStage = "powered on"
	nodeStageReady      nodeStartStage = "ready"
	nodeStageConfigured nodeStartStage = "configured"
)

var startNodes = &cobra.Command{
	Use:                   "start",
	Short:                 "Power-on and configure nodes",
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		nodes := cfg.GetNodes()
		client := clientPkg.NewClient(cfg)

		stages := startNodesSequence(cmd.Context(), &client, nodes)
		if !isSuccesStartSequence(stages) {
			log.Error("Nodes start drifted")
			for name, stage := range stages {
				if stage == nodeStageConfigured {
					log.Success(" %s: %s", color.CyanString(name), stage)
				} else {
					log.Warn(" %s: %s (expected %s)", color.CyanString(name), stage, nodeStageConfigured)
				}
			}

		} else {
			log.Success("Nodes successfully started")
		}
	},
}

func isSuccesStartSequence(stages map[string]nodeStartStage) bool {
	for _, stage := range stages {
		if stage != nodeStageConfigured {
			return false
		}
	}
	return true
}

// startNodesSequence powers on, waits for and configures the given nodes,
// stopping at the first failure.
// Returns the last report start stages, if all nodes are nodeStageConfigured then it's success
func startNodesSequence(ctx context.Context, client *clientPkg.Client, nodes map[string]*common.Node) map[string]nodeStartStage {
	stages := make(map[string]nodeStartStage, len(nodes))
	for name := range nodes {
		stages[name] = nodeStagePending
	}

	names := slices.Sorted(maps.Keys(nodes))

	for _, name := range names {
		if err := client.PowerOnNode(nodes[name]); err != nil {
			log.Warn("Powering on %s failed: %s", color.CyanString(name), err.Error())
			return stages
		}
		stages[name] = nodeStagePoweredOn
	}

	waitCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	ready := true
	client.WaitNodesReady(waitCtx, nodes, func(n *common.Node, err error) {
		if err == nil {
			stages[n.Name] = nodeStageReady
		} else if ready {
			log.Warn("Waiting for %s failed: %s", color.CyanString(n.Name), err.Error())
			ready = false
			cancel()
		}
	})
	if !ready {
		return stages
	}

	toLoad := make([]string, 0, len(names))
	for _, name := range names {
		if err := client.ValidateNodeConfig(nodes[name]); err == nil {
			toLoad = append(toLoad, name)
		} else if errors.Is(err, common.ErrNodeAlreadyConfigured) {
			stages[name] = nodeStageConfigured
			log.Info("Loading %s skipped: %s", color.CyanString(name), err.Error())
		} else {
			log.Warn("Validating %s failed: %s", color.CyanString(name), err.Error())
			return stages
		}
	}

	for _, name := range toLoad {
		if err := client.LoadNodeConfig(nodes[name]); err != nil {
			log.Warn("Loading %s failed: %s", color.CyanString(name), err.Error())
			return stages
		}
		stages[name] = nodeStageConfigured
	}

	return stages
}

var powerOnNodes = &cobra.Command{
	Use:                   "poweron [--all | node...]",
	Short:                 "Power-on nodes",
	Args:                  validateNodeArgs,
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, nodeNames []string) {
		nodes := cfg.GetNodes(nodeNames...)
		client := clientPkg.NewClient(cfg)
		client.PowerOnNodes(nodes, func(n *common.Node, err error) {
			if err == nil {
				log.Success("Powering on %s completed", color.CyanString(n.Name))
			} else {
				log.Warn("Powering on %s failed: %s", color.CyanString(n.Name), err.Error())
			}
		})
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
		client.PowerOffNodes(nodes, func(n *common.Node, err error) {
			if err == nil {
				log.Success("Powering off %s completed", color.CyanString(n.Name))
			} else {
				log.Warn("Powering off %s failed: %s", color.CyanString(n.Name), err.Error())
			}
		})
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
		client.RestartNodes(nodes, "", 0, func(n *common.Node, err error) {
			if err == nil {
				log.Success("Restarting %s completed", color.CyanString(n.Name))
			} else {
				log.Warn("Restarting %s failed: %s", color.CyanString(n.Name), err.Error())
			}
		})
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
		client.ShutdownNodes(nodes, "", 0, func(n *common.Node, err error) {
			if err == nil {
				log.Success("Shutting down %s completed", color.CyanString(n.Name))
			} else {
				log.Warn("Shutting down %s failed: %s", color.CyanString(n.Name), err.Error())
			}
		})
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
	listNodes.Flags().Bool("json", false, "Output as JSON")
	listNodes.Flags().Duration("timeout", 2*time.Second, "Time to wait for mDNS discovery")
	startNodes.Flags().Bool("all", false, "All nodes")
	powerOnNodes.Flags().Bool("all", false, "All nodes")
	powerOffNodes.Flags().Bool("all", false, "All nodes")
	shutdownNodes.Flags().Bool("all", false, "All nodes")
	restartNodes.Flags().Bool("all", false, "All nodes")

	hostCmd.AddCommand(listNodes)
	hostCmd.AddCommand(nodeStates)
	hostCmd.AddCommand(configCmd)
	hostCmd.AddCommand(startNodes)
	hostCmd.AddCommand(powerOnNodes)
	hostCmd.AddCommand(powerOffNodes)
	hostCmd.AddCommand(shutdownNodes)
	hostCmd.AddCommand(restartNodes)

	rootCmd.AddCommand(hostCmd)
}
