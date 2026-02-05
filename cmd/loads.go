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
	"github.com/bitomia/realm/common/config"
	"github.com/bitomia/realm/common/dto"
)

func doPlanLoads(client *clientPkg.Client, loads map[string]*common.Load) error {
	if len(loads) == 0 {
		return fmt.Errorf("No loads")
	}
	for _, load := range loads {
		log.Info(" -> Planning load %s", color.CyanString(load.Name))
		if err := client.PlanLoad(load); err != nil {
			return fmt.Errorf("Error planning load: %s", err.Error())
		}
	}
	return nil
}

func validateLoadArgs(cmd *cobra.Command, args []string) error {
	all, _ := cmd.Flags().GetBool("all")

	if all && len(args) > 0 {
		return fmt.Errorf("Cannot use --all with load names")
	}

	if !all && len(args) == 0 {
		return fmt.Errorf("Must specify --all or at least one load name")
	}

	return nil
}

var loadsCmd = &cobra.Command{
	Use:                   "loads",
	Aliases:               []string{"l"},
	Short:                 "Interface with loads",
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Realm CLI. Use -h for help.")
	},
}

var planLoads = &cobra.Command{
	Use:                   "plan [--all | load...]",
	Short:                 "Plan loads on nodes",
	Args:                  validateLoadArgs,
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, loadNames []string) {
		loads := config.GetLoadsFromConfig(loadNames...)
		client := clientPkg.NewClient()
		if err := doPlanLoads(&client, loads); err != nil {
			log.Fatal("Error planning load: %s", err.Error())
		}
		log.Info("Successfully verified loads on cluster")
	},
}

var listLoads = &cobra.Command{
	Use:                   "list [--all | load...]",
	Aliases:               []string{"ls"},
	Short:                 "List loads deployments",
	Args:                  validateLoadArgs,
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, loadNames []string) {
		loads := config.GetLoadsFromConfig(loadNames...)
		if len(loads) == 0 {
			log.Error("No loads")
			return
		}
		client := clientPkg.NewClient()

		// Collect load deployments from all nodes
		nodeLoadsDeployments := make(map[string]map[string]dto.LoadsDeployments)
		for _, load := range loads {
			if _, exists := nodeLoadsDeployments[load.Node.Url]; !exists {
				loadsDeployments, err := client.GetLoadsDeployments(load.Node.Url)
				if err != nil {
					log.Error(err.Error())
					nodeLoadsDeployments[load.Node.Url] = nil // Mark as failed
				} else {
					nodeLoadsDeployments[load.Node.Url] = make(map[string]dto.LoadsDeployments)
					for _, s := range loadsDeployments {
						nodeLoadsDeployments[load.Node.Url][s.LoadName] = loadsDeployments
					}
				}
			}
		}

		for _, load := range loads {
			stateStr := ""
			containerNames := []string{}
			if nodeLoadsDeployments[load.Node.Url] == nil {
				stateStr = color.RedString("unknown")
			} else {
				deployments, exists := nodeLoadsDeployments[load.Node.Url][load.Name]
				if !exists {
					stateStr = color.WhiteString("not deployed")
				} else {
					for _, d := range deployments {
						// Extract container name from metadata if it's a container driver
						if d.Driver == "container" && d.Metadata != nil {
							var metadata map[string]interface{}
							if metadataBytes, err := json.Marshal(d.Metadata); err == nil {
								if err := json.Unmarshal(metadataBytes, &metadata); err == nil {
									if containerName, ok := metadata["container_name"].(string); ok && containerName != "" {
										containerNames = append(containerNames, containerName)
									}
								}
							}
						}
						switch d.State {
						case "running":
							stateStr = fmt.Sprintf("%s %s", stateStr, color.GreenString("running"))
						case "planned":
							stateStr = fmt.Sprintf("%s %s", stateStr, color.YellowString("planned"))
						default:
							stateStr = fmt.Sprintf("%s %s", stateStr, string(d.State))
						}
					}
				}
			}

			containerInfo := ""
			if len(containerNames) > 0 {
				containerInfo = fmt.Sprintf(" [%s]", strings.Join(containerNames, ", "))
			}
			color.White("%s (node %s) [%s]%s\n", color.CyanString(load.Name), color.YellowString(load.Node.Name), strings.TrimSpace(stateStr), containerInfo)
			prettyJSON(load, "name", "node")
		}
	},
}

var startLoads = &cobra.Command{
	Use:                   "start [--all | load...]",
	Short:                 "Start loads (must be planned first)",
	Args:                  validateLoadArgs,
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, loadNames []string) {
		loads := config.GetLoadsFromConfig(loadNames...)
		client := clientPkg.NewClient()

		// Planning must be done separately with 'plan' command

		loaded := make(map[string]bool)
		for _, l := range loads {
			startChain := l.StartChain
			for _, l := range startChain {
				if _, exists := loaded[l.Name]; !exists {
					loaded[l.Name] = true
					log.Info(" -> Starting load %s", color.CyanString(l.Name))
					if err := client.StartLoad(l); err != nil {
						log.Fatal("Starting load failed: %s", err.Error())
					}
				}
			}
		}
	},
}

var stopLoads = &cobra.Command{
	Use:                   "stop [--all | load...]",
	Short:                 "Stop loads",
	Args:                  validateLoadArgs,
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, loadNames []string) {
		loads := config.GetLoadsFromConfig(loadNames...)
		client := clientPkg.NewClient()
		stopped := make(map[string]bool)

		for _, l := range loads {
			stopChain := l.StopChain
			for _, l := range stopChain {
				if _, exists := stopped[l.Name]; !exists {
					stopped[l.Name] = true
					log.Info(" -> Stopping load %s", color.CyanString(l.Name))
					if err := client.StopLoad(l); err != nil {
						log.Fatal("Stopping load failed: %s", err.Error())
					}
				}
			}
		}
	},
}

var unplanLoads = &cobra.Command{
	Use:                   "unplan [--all | load...]",
	Short:                 "Remove planned (not started) loads",
	Args:                  validateLoadArgs,
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, loadNames []string) {
		loads := config.GetLoadsFromConfig(loadNames...)
		client := clientPkg.NewClient()
		unplanned := make(map[string]bool)

		for _, l := range loads {
			if _, exists := unplanned[l.Name]; !exists {
				unplanned[l.Name] = true
				log.Info(" -> Unplanning load %s", color.CyanString(l.Name))
				if err := client.UnplanLoad(l); err != nil {
					log.Fatal("Unplanning load failed: %s", err.Error())
				}
			}
		}
	},
}

var graphLoads = &cobra.Command{
	Use:                   "graph",
	Short:                 "Print the dependency graph",
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, loadNames []string) {
		g := config.GetLoadsConfigGraph()
		adjacencyMap, err := g.AdjacencyMap()
		if err != nil {
			log.Fatal("%s", err.Error())
		}

		for v, adjacencies := range adjacencyMap {
			if len(adjacencies) == 0 {
				fmt.Printf("%v\n", v)
			} else {
				for a := range adjacencies {
					fmt.Printf("%v -> %v\n", v, a)
				}
			}

		}
	},
}

var stdoutLoad = &cobra.Command{
	Use:                   "stdout <load>",
	Short:                 "Read stdout from a load",
	Args:                  cobra.ExactArgs(1),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, loadNames []string) {
		loads := config.GetLoadsFromConfig(loadNames...)
		if len(loads) == 0 {
			log.Fatal("Load not found")
		}
		if len(loads) > 1 {
			log.Fatal("Multiple loads found with that name")
		}

		load := loads[loadNames[0]]
		client := clientPkg.NewClient()
		if err := client.ReadLoadStdout(load); err != nil {
			log.Fatal("Failed to read stdout: %s", err.Error())
		}
	},
}

var stderrLoad = &cobra.Command{
	Use:                   "stderr <load>",
	Short:                 "Read stderr from a load",
	Args:                  cobra.ExactArgs(1),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, loadNames []string) {
		loads := config.GetLoadsFromConfig(loadNames...)
		if len(loads) == 0 {
			log.Fatal("Load not found")
		}
		if len(loads) > 1 {
			log.Fatal("Multiple loads found with that name")
		}

		load := loads[loadNames[0]]
		client := clientPkg.NewClient()
		if err := client.ReadLoadStderr(load); err != nil {
			log.Fatal("Failed to read stderr: %s", err.Error())
		}
	},
}

func init() {
	startLoads.Flags().Bool("all", false, "All loads (cluster mode)")
	planLoads.Flags().Bool("all", false, "All loads (cluster mode)")
	listLoads.Flags().Bool("all", false, "All loads (cluster mode)")
	stopLoads.Flags().Bool("all", false, "All loads (cluster mode)")
	unplanLoads.Flags().Bool("all", false, "All loads (cluster mode)")

	loadsCmd.AddCommand(graphLoads)
	loadsCmd.AddCommand(listLoads)
	loadsCmd.AddCommand(planLoads)
	loadsCmd.AddCommand(startLoads)
	loadsCmd.AddCommand(stdoutLoad)
	loadsCmd.AddCommand(stderrLoad)
	loadsCmd.AddCommand(stopLoads)
	loadsCmd.AddCommand(unplanLoads)
	rootCmd.AddCommand(loadsCmd)
}
