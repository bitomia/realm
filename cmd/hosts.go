package main

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/bitomia/realm/internal/config"
)

var hostCmd = &cobra.Command{
	Use:     "hosts",
	Aliases: []string{"h"},
	Short:   "Interface with hosts",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Realm CLI. Use -h for help.")
	},
}

var hostStatus = &cobra.Command{
	Use:                   "status",
	Short:                 "Get host status",
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		client := NewClient()
		hosts := config.Get().Daemons

		for _, host := range hosts {
			status, err := client.GetHostStatus(host)
			if err == nil {
				color.Blue("Host: %s\n", color.CyanString(host))

				// CPU Information
				fmt.Printf("Number CPU: %s\n", color.CyanString(fmt.Sprintf("%d", status.NumCPU)))
				fmt.Printf("CPU Usage: %s%%\n", color.CyanString(fmt.Sprintf("%.2f", status.UsageCPUPercent)))
				fmt.Printf("User CPU: %s\n", color.CyanString(fmt.Sprintf("%d", status.UserCPU)))
				fmt.Printf("System CPU: %s\n", color.CyanString(fmt.Sprintf("%d", status.SystemCPU)))
				fmt.Printf("Idle CPU: %s\n", color.CyanString(fmt.Sprintf("%d", status.IdleCPU)))
				fmt.Printf("Total CPU: %s\n", color.CyanString(fmt.Sprintf("%d", status.TotalCPU)))

				// Memory Information
				fmt.Printf("Memory Usage: %s%%\n", color.CyanString(fmt.Sprintf("%.2f", float64(status.UsedMem)/float64(status.TotalMem)*100)))
				fmt.Printf("Free Memory: %s%%\n", color.CyanString(fmt.Sprintf("%.2f", status.FreeMemPercent)))
				fmt.Printf("Total Memory: %s MB\n", color.CyanString(fmt.Sprintf("%.2f", float64(status.TotalMem)/(1024.0*1024.0))))
				fmt.Printf("Used Memory: %s MB\n", color.CyanString(fmt.Sprintf("%.2f", float64(status.UsedMem)/(1024.0*1024.0))))
				fmt.Printf("Available Memory: %s MB\n", color.CyanString(fmt.Sprintf("%.2f", float64(status.AvailableMem)/(1024.0*1024.0))))
				fmt.Printf("Free Memory: %s MB\n", color.CyanString(fmt.Sprintf("%.2f", float64(status.FreeMem)/(1024.0*1024.0))))
				fmt.Printf("Cached Memory: %s MB\n", color.CyanString(fmt.Sprintf("%.2f", float64(status.CachedMem)/(1024.0*1024.0))))
				fmt.Printf("Inactive Memory: %s MB\n", color.CyanString(fmt.Sprintf("%.2f", float64(status.InactiveMem)/(1024.0*1024.0))))

				// Storage Information
				fmt.Printf("Free Storage: %s MB\n", color.CyanString(fmt.Sprintf("%.2f", float64(status.FreeStorage)/(1024.0*1024.0))))

				// Container Information
				if len(status.Containers) > 0 {
					fmt.Printf("Containers (%d):\n", len(status.Containers))
					for _, container := range status.Containers {
						fmt.Printf("  - %s:\n", color.YellowString(container.ContainerID))
						fmt.Printf("    CPU Usage: %s%%\n", color.CyanString(fmt.Sprintf("%.2f", container.CPUUsage)))
						fmt.Printf("    Memory Usage: %s%% (%s MB)\n",
							color.CyanString(fmt.Sprintf("%.2f", container.MemoryPercent)),
							color.CyanString(fmt.Sprintf("%.2f", container.MemoryUsage/(1024.0*1024.0))))
						fmt.Printf("    Memory Limit: %s MB\n", color.CyanString(fmt.Sprintf("%.2f", container.MemoryLimit/(1024.0*1024.0))))
					}
				} else {
					fmt.Printf("Containers: %s\n", color.CyanString("0"))
				}

				fmt.Println()
			} else {
				color.Red("Error getting status for host %s: %v\n", host, err)
			}
		}
	},
}

func init() {
	hostCmd.AddCommand(hostStatus)
	rootCmd.AddCommand(hostCmd)
}
