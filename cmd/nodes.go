package main

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	clientPkg "github.com/bitomia/realm/cmd/client"
	"github.com/bitomia/realm/cmd/log"

	"github.com/bitomia/realm/internal"
)

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
			if node.MAC != nil {
				fmt.Printf(" MAC: %s\n", color.CyanString(*node.MAC))
			}
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

func init() {
	hostCmd.AddCommand(nodeStates)
	rootCmd.AddCommand(hostCmd)
}
