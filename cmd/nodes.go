package main

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	cmdInternal "github.com/bitomia/realm/cmd/internal"
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
		client := cmdInternal.NewClient()

		for id, node := range cmdInternal.GetNodes() {
			fmt.Printf("Node: %s\n", color.CyanString(id))
			fmt.Printf(" URL: %s\n", color.CyanString(node.Url))
			if node.MAC != nil {
				fmt.Printf(" MAC: %s\n", color.CyanString(*node.MAC))
			}
			status, err := client.GetNodeState(node.Url)
			if err != nil {
				log.Info(" Node not available: %s", err.Error())
			} else {
				log.Info(" CPUs count: %s", color.CyanString(fmt.Sprintf("%d", status.NumCPU)))
				log.Info(" CPU Usage: %s%%", color.CyanString(fmt.Sprintf("%.2f", status.UsageCPUPercent)))
				log.Info(" User CPU: %s", color.CyanString(fmt.Sprintf("%d", status.UserCPU)))
				log.Info(" System CPU: %s", color.CyanString(fmt.Sprintf("%d", status.SystemCPU)))
				log.Info(" Idle CPU: %s", color.CyanString(fmt.Sprintf("%d", status.IdleCPU)))
				log.Info(" Total CPU: %s", color.CyanString(fmt.Sprintf("%d", status.TotalCPU)))

				log.Info(" Memory Usage: %s%%", color.CyanString(fmt.Sprintf("%.2f", float64(status.UsedMem)/float64(status.TotalMem)*100)))
				log.Info(" Free Memory: %s%%", color.CyanString(fmt.Sprintf("%.2f", status.FreeMemPercent)))
				log.Info(" Total Memory: %s MB", color.CyanString(fmt.Sprintf("%.2f", internal.ToMB(float64(status.TotalMem)))))
				log.Info(" Used Memory: %s MB", color.CyanString(fmt.Sprintf("%.2f", internal.ToMB(float64(status.UsedMem)))))
				log.Info(" Free Memory: %s MB", color.CyanString(fmt.Sprintf("%.2f", internal.ToMB(float64(status.FreeMem)))))
				log.Info(" Free Storage: %s MB", color.CyanString(fmt.Sprintf("%.2f", internal.ToMB(float64(status.FreeStorage)))))

				if len(status.Containers) > 0 {
					log.Info("Containers (%d):", len(status.Containers))
					for _, container := range status.Containers {
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
