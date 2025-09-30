package main

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

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
		client := internal.NewClient()

		for _, node := range internal.GetNodes() {
			status, err := client.GetNodeState(node.Url.String())
			if err == nil {
				color.Blue("Node: %s\n", color.CyanString(node.Name))
				color.Blue("URL: %s\n", color.CyanString(node.Url.String()))

				log.Info("CPUs count: %s", color.CyanString(fmt.Sprintf("%d", status.NumCPU)))
				log.Info("CPU Usage: %s%%", color.CyanString(fmt.Sprintf("%.2f", status.UsageCPUPercent)))
				log.Info("User CPU: %s", color.CyanString(fmt.Sprintf("%d", status.UserCPU)))
				log.Info("System CPU: %s", color.CyanString(fmt.Sprintf("%d", status.SystemCPU)))
				log.Info("Idle CPU: %s", color.CyanString(fmt.Sprintf("%d", status.IdleCPU)))
				log.Info("Total CPU: %s", color.CyanString(fmt.Sprintf("%d", status.TotalCPU)))

				log.Info("Memory Usage: %s%%", color.CyanString(fmt.Sprintf("%.2f", float64(status.UsedMem)/float64(status.TotalMem)*100)))
				log.Info("Free Memory: %s%%", color.CyanString(fmt.Sprintf("%.2f", status.FreeMemPercent)))
				log.Info("Total Memory: %s MB", color.CyanString(fmt.Sprintf("%.2f", float64(status.TotalMem)/(1024.0*1024.0))))
				log.Info("Used Memory: %s MB", color.CyanString(fmt.Sprintf("%.2f", float64(status.UsedMem)/(1024.0*1024.0))))
				log.Info("Available Memory: %s MB", color.CyanString(fmt.Sprintf("%.2f", float64(status.AvailableMem)/(1024.0*1024.0))))
				log.Info("Free Memory: %s MB", color.CyanString(fmt.Sprintf("%.2f", float64(status.FreeMem)/(1024.0*1024.0))))
				log.Info("Cached Memory: %s MB", color.CyanString(fmt.Sprintf("%.2f", float64(status.CachedMem)/(1024.0*1024.0))))
				log.Info("Inactive Memory: %s MB", color.CyanString(fmt.Sprintf("%.2f", float64(status.InactiveMem)/(1024.0*1024.0))))

				log.Info("Free Storage: %s MB", color.CyanString(fmt.Sprintf("%.2f", float64(status.FreeStorage)/(1024.0*1024.0))))

				if len(status.Containers) > 0 {
					log.Info("Containers (%d):", len(status.Containers))
					for _, container := range status.Containers {
						log.Info("  - %s:", color.YellowString(container.ContainerID))
						log.Info("    CPU Usage: %s%%", color.CyanString(fmt.Sprintf("%.2f", container.CPUUsage)))
						log.Info("    Memory Usage: %s%% (%s MB)",
							color.CyanString(fmt.Sprintf("%.2f", container.MemoryPercent)),
							color.CyanString(fmt.Sprintf("%.2f", container.MemoryUsage/(1024.0*1024.0))))
						log.Info("    Memory Limit: %s MB", color.CyanString(fmt.Sprintf("%.2f", container.MemoryLimit/(1024.0*1024.0))))
					}
				} else {
					log.Info("Containers: %s", color.CyanString("0"))
				}

				fmt.Println()
			} else {
				log.Error("Error retrieving node state %v: %v", node, err)
			}
		}
	},
}

func init() {
	hostCmd.AddCommand(nodeStates)
	rootCmd.AddCommand(hostCmd)
}
