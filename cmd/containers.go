package main

import (
	"fmt"

	"github.com/bitomia/realm/cmd/log"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var containersCmd = &cobra.Command{
	Use:                   "containers",
	Aliases:               []string{"c"},
	Short:                 "Interface with containers",
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Realm CLI. Use -h for help.")
	},
}

var listContainers = &cobra.Command{
	Use:                   "ls",
	Short:                 "List all available containers",
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		client := NewClient()
		containersPerHost, err := client.GetAllContainers()
		if err != nil {
			log.Error("Error %v\n", err)
			return
		}
		for host, containers := range containersPerHost {
			color.Blue("Containers in %s\n", color.CyanString(host))
			for _, c := range containers {
				log.Info("- %s\n", color.CyanString(fmt.Sprintf("%v", c)))
			}
		}
	},
}

var createContainer = &cobra.Command{
	Use:                   "create [daemon] [image] [container]",
	Short:                 "Create a container",
	Args:                  cobra.ExactArgs(3),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		client := NewClient()
		color.Blue("Creating container %s on %s with image %s\n", color.CyanString(args[2]), color.CyanString(args[0]), color.CyanString(args[1]))
		client.CreateContainer(args[0], args[1], args[2])
		color.Green("Successfully created container %s\n", color.CyanString(args[2]))
	},
}

var startContainer = &cobra.Command{
	Use:                   "start [host] [container]",
	Short:                 "Start a container",
	DisableFlagsInUseLine: true,
	Args:                  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		client := NewClient()
		color.Blue("Starting container %s on %s\n", color.CyanString(args[1]), color.CyanString(args[0]))
		client.StartContainer(args[0], args[1])
		color.Green("Successfully started container %s\n", color.CyanString(args[1]))
	},
}

var stopContainer = &cobra.Command{
	Use:                   "stop [host] [container]",
	Short:                 "Stop a container",
	DisableFlagsInUseLine: true,
	Args:                  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		client := NewClient()
		color.Blue("Stopping container %s on %s\n", color.CyanString(args[1]), color.CyanString(args[0]))
		client.StopContainer(args[0], args[1])
		color.Green("Successfully stopped container %s\n", color.CyanString(args[1]))
	},
}

var deleteContainer = &cobra.Command{
	Use:                   "delete [host] [container]",
	Short:                 "Delete a container",
	DisableFlagsInUseLine: true,
	Args:                  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		client := NewClient()
		color.Blue("Deleting container %s on %s\n", color.CyanString(args[1]), color.CyanString(args[0]))
		client.DeleteContainer(args[0], args[1])
		color.Green("Successfully deleted container %s\n", color.CyanString(args[1]))
	},
}

var updateQuotas = &cobra.Command{
	Use:                   "quotas [host] [container] --cpu [cpu_quota] --memory [memory_limit] --volume [volume_size]",
	Short:                 "Update container resource quotas",
	Args:                  cobra.ExactArgs(2),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		client := NewClient()
		cpuQuota, _ := cmd.Flags().GetInt64("cpu")
		memoryLimit, _ := cmd.Flags().GetInt64("memory")
		volumeSize, _ := cmd.Flags().GetInt64("volume")

		color.Blue("Updating quotas for container %s on %s\n", color.CyanString(args[1]), color.CyanString(args[0]))
		if err := client.UpdateContainerQuotas(args[0], args[1], cpuQuota, memoryLimit, volumeSize); err != nil {
			color.Red("Error updating quotas: %v\n", err)
		} else {
			color.Green("Successfully updated quotas for container %s\n", color.CyanString(args[1]))
		}
	},
}

var repairContainer = &cobra.Command{
	Use:                   "repair [host] [container]",
	Short:                 "Repair a container to restore its previous state",
	Args:                  cobra.ExactArgs(2),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		client := NewClient()
		color.Blue("Repairing container %s on %s\n", color.CyanString(args[1]), color.CyanString(args[0]))
		if err := client.RepairContainer(args[0], args[1]); err != nil {
			color.Red("Error repairing container: %v\n", err)
		} else {
			color.Green("Successfully repaired container %s\n", color.CyanString(args[1]))
		}
	},
}

var sendSignal = &cobra.Command{
	Use:                   "signal [host] [container] [signal]",
	Short:                 "Send a system signal to a container",
	Args:                  cobra.ExactArgs(3),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		client := NewClient()
		color.Blue("Sending signal %s to container %s on %s\n", color.CyanString(args[2]), color.CyanString(args[1]), color.CyanString(args[0]))
		if err := client.SendContainerSignal(args[0], args[1], args[2]); err != nil {
			color.Red("Error sending signal: %v\n", err)
		} else {
			color.Green("Successfully sent signal %s to container %s\n", color.CyanString(args[2]), color.CyanString(args[1]))
		}
	},
}

var migrateContainer = &cobra.Command{
	Use:                   "migrate [host] [container] [new_image]",
	Short:                 "Migrate container to new image while preserving state",
	Args:                  cobra.ExactArgs(3),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		client := NewClient()
		color.Blue("Migrating container %s on %s to image %s\n", color.CyanString(args[1]), color.CyanString(args[0]), color.CyanString(args[2]))
		if err := client.MigrateContainer(args[0], args[1], args[2]); err != nil {
			color.Red("Error migrating container: %v\n", err)
		} else {
			color.Green("Successfully migrated container %s to image %s\n", color.CyanString(args[1]), color.CyanString(args[2]))
		}
	},
}

var getLogs = &cobra.Command{
	Use:                   "logs [host] [container]",
	Short:                 "Get container logs",
	Args:                  cobra.ExactArgs(2),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		client := NewClient()
		color.Blue("Getting logs for container %s on %s\n", color.CyanString(args[1]), color.CyanString(args[0]))
		if err := client.GetContainerLogs(args[0], args[1]); err != nil {
			color.Red("Error getting logs: %v\n", err)
		}
	},
}

func init() {
	// Add flags for quotas command
	updateQuotas.Flags().Int64("cpu", 0, "CPU quota limit")
	updateQuotas.Flags().Int64("memory", 0, "Memory limit in bytes")
	updateQuotas.Flags().Int64("volume", 0, "Volume size limit in bytes")

	containersCmd.AddCommand(listContainers)
	containersCmd.AddCommand(createContainer)
	containersCmd.AddCommand(stopContainer)
	containersCmd.AddCommand(startContainer)
	containersCmd.AddCommand(deleteContainer)
	containersCmd.AddCommand(updateQuotas)
	containersCmd.AddCommand(repairContainer)
	containersCmd.AddCommand(sendSignal)
	containersCmd.AddCommand(migrateContainer)
	containersCmd.AddCommand(getLogs)
	containersCmd.DisableFlagsInUseLine = true
	rootCmd.AddCommand(containersCmd)
}
