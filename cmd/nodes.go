package main

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	clientPkg "github.com/bitomia/realm/cmd/client"
	"github.com/bitomia/realm/cmd/log"
	"github.com/bitomia/realm/common/config"
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
	Use:                   "ls",
	Short:                 "List and retrieve all node states",
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		client := clientPkg.NewClient()

		for id, node := range clientPkg.GetNodes() {
			fmt.Printf("Node: %s\n", color.CyanString(id))
			fmt.Printf(" URL: %s\n", color.CyanString(node.Url))
			state, err := client.GetNodeState(node.Url)
			if err != nil {
				log.Info(" Node not available: %s", err.Error())
			} else {
				log.Info(" CPUs count: %s", color.CyanString(fmt.Sprintf("%d", state.NumCPU)))
				log.Info(" CPU Usage: %s%%", color.CyanString(fmt.Sprintf("%.2f", state.UsageCPUPercent)))
				log.Info(" User CPU: %s", color.CyanString(fmt.Sprintf("%d", state.UserCPU)))
				log.Info(" System CPU: %s", color.CyanString(fmt.Sprintf("%d", state.SystemCPU)))
				log.Info(" Idle CPU: %s", color.CyanString(fmt.Sprintf("%d", state.IdleCPU)))
				log.Info(" Total CPU: %s", color.CyanString(fmt.Sprintf("%d", state.TotalCPU)))

				log.Info(" Memory Usage: %s%%", color.CyanString(fmt.Sprintf("%.2f", float64(state.UsedMem)/float64(state.TotalMem)*100)))
				log.Info(" Free Memory: %s%%", color.CyanString(fmt.Sprintf("%.2f", state.FreeMemPercent)))
				log.Info(" Total Memory: %s MB", color.CyanString(fmt.Sprintf("%.2f", internal.ToMB(float64(state.TotalMem)))))
				log.Info(" Used Memory: %s MB", color.CyanString(fmt.Sprintf("%.2f", internal.ToMB(float64(state.UsedMem)))))
				log.Info(" Free Memory: %s MB", color.CyanString(fmt.Sprintf("%.2f", internal.ToMB(float64(state.FreeMem)))))
				log.Info(" Free Storage: %s MB", color.CyanString(fmt.Sprintf("%.2f", internal.ToMB(float64(state.FreeStorage)))))

				if len(state.Containers) > 0 {
					log.Info("Containers (%d):", len(state.Containers))
					for _, container := range state.Containers {
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

				fmt.Println()
			}
		}
	},
}

var planNodes = &cobra.Command{
	Use:                   "plan [--all | node...]",
	Short:                 "Plan nodes on the cluster",
	Args:                  validateNodeArgs,
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, nodeNames []string) {
		nodes := config.GetNodesFromConfig(nodeNames...)
		client := clientPkg.NewClient()

		for _, n := range nodes {
			log.Info(" -> Planning node %s", color.CyanString(n.Name))
			if err := client.PlanNode(n); err != nil {
				log.Fatal("Planning node '%s' failed: %s", n.Name, err.Error())
			}
		}
	},
}

var startNodes = &cobra.Command{
	Use:                   "start [--all | node...]",
	Short:                 "Startup nodes",
	Args:                  validateNodeArgs,
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, nodeNames []string) {
		nodes := config.GetNodesFromConfig(nodeNames...)
		client := clientPkg.NewClient()

		for _, n := range nodes {
			log.Info(" -> Starting up node %s", color.CyanString(n.Name))
			if err := client.StartupNode(n); err != nil {
				log.Fatal("Starting node '%s' failed: %s", n.Name, err.Error())
			}
		}
	},
}

var shutdownNodes = &cobra.Command{
	Use:                   "shutdown [--all | load...]",
	Short:                 "Stop loads",
	Args:                  validateLoadArgs,
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, loadNames []string) {
		nodes := config.GetNodesFromConfig(loadNames...)
		client := clientPkg.NewClient()

		for _, n := range nodes {
			log.Info(" -> Shutting down node %s", color.CyanString(n.Name))
			if err := client.ShutdownNode(n, "", 0); err != nil {
				log.Fatal("Shutting down node '%s' failed: %s", n.Name, err.Error())
			}
		}
	},
}

func init() {
	planNodes.Flags().Bool("all", false, "All nodes")
	startNodes.Flags().Bool("all", false, "All nodes")
	shutdownNodes.Flags().Bool("all", false, "All nodes")

	hostCmd.AddCommand(nodeStates)
	hostCmd.AddCommand(planNodes)
	hostCmd.AddCommand(startNodes)
	hostCmd.AddCommand(shutdownNodes)
	rootCmd.AddCommand(hostCmd)
}
