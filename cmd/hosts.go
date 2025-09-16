package main

import (
	"fmt"

	"github.com/bitomia/realm/cmd/log"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
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

		daemons := GetDaemonAddresses()

		for _, daemon := range daemons {
			status, err := client.GetHostStatus(daemon.Url)
			if err == nil {
				color.Blue("Host: %s\n", color.CyanString(daemon.Name))
				color.Blue("URL: %s\n", color.CyanString(daemon.Url))

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
				log.Error("Error getting status for host %s: %v", daemon, err)
			}
		}
	},
}

func init() {
	hostCmd.AddCommand(hostStatus)
	rootCmd.AddCommand(hostCmd)
}
