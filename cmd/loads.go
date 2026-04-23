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
)

func doProvisionLoads(client *clientPkg.Client, loads map[string]*common.Load) error {
	if len(loads) == 0 {
		return fmt.Errorf("No loads")
	}
	for _, load := range loads {
		log.Info("Provisioning load %s", color.CyanString(load.Name))
		if err := client.ProvisionLoad(load); err != nil {
			return err
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

var provisionLoads = &cobra.Command{
	Use:                   "provision [--all | load...]",
	Short:                 "Provision loads on nodes",
	Args:                  validateLoadArgs,
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, loadNames []string) {
		loads := cfg.GetLoads(loadNames...)
		client := clientPkg.NewClient(cfg)
		if err := doProvisionLoads(&client, loads); err != nil {
			log.Warn("Error provisioning load: %s", err.Error())
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
		loads := cfg.GetLoads(loadNames...)
		if len(loads) == 0 {
			log.Error("No loads")
			return
		}
		client := clientPkg.NewClient(cfg)

		// Collect load deployments from all nodes
		nodeLoadsDeployments := make(map[string]map[string]dto.LoadsDeployments)
		for _, load := range loads {
			if _, exists := nodeLoadsDeployments[load.Node.Url]; !exists {
				loadsDeployments, err := client.GetLoadsDeployments(load.Node.Url)

				if err != nil {
					log.Error("%s", err.Error())
					nodeLoadsDeployments[load.Node.Url] = nil // Mark as failed
				} else {
					nodeLoadsDeployments[load.Node.Url] = make(map[string]dto.LoadsDeployments)
					for _, s := range loadsDeployments {
						nodeLoadsDeployments[load.Node.Url][s.LoadName] = append(nodeLoadsDeployments[load.Node.Url][s.LoadName], s)
					}
				}
			}
		}

		for _, load := range loads {
			deploymentStatusStr := "unknown"
			containerNames := []string{}

			if nodeLoadsDeployments[load.Node.Url] != nil {
				deployments, exists := nodeLoadsDeployments[load.Node.Url][load.Name]
				if !exists {
					deploymentStatusStr = "not deployed"
				} else {
					for _, d := range deployments {
						// Extract container name from metadata if it's a container driver
						if d.Driver == "container" && d.Metadata != nil {
							var metadata map[string]any
							if metadataBytes, err := json.Marshal(d.Metadata); err == nil {
								if err := json.Unmarshal(metadataBytes, &metadata); err == nil {
									if containerName, ok := metadata["container_name"].(string); ok && containerName != "" {
										containerNames = append(containerNames, containerName)
									}
								}
							}
						}

						switch d.DeploymentStatus.StatusCode {
						case common.DeploymentStatusRunning:
							deploymentStatusStr = color.GreenString("running")
						case common.DeploymentStatusReady:
							deploymentStatusStr = color.HiBlueString("ready")
						case common.DeploymentStatusStopped:
							deploymentStatusStr = color.YellowString("stopped")
						case common.DeploymentStatusError:
							deploymentStatusStr = fmt.Sprintf("%s %s", color.RedString("error"), d.DeploymentStatus.Reason)
						default:
							deploymentStatusStr = string(d.DeploymentStatus.StatusCode)
						}
					}
				}
			}

			containerInfo := ""
			if len(containerNames) > 0 {
				containerInfo = fmt.Sprintf(" [%s]", strings.Join(containerNames, ", "))
			}
			color.White("%s (node %s) [%s]%s\n", color.CyanString(load.Name), color.YellowString(load.Node.Name), strings.TrimSpace(deploymentStatusStr), containerInfo)
			prettyJSON(load, "name", "node")
		}
	},
}

var startLoads = &cobra.Command{
	Use:                   "start [--all | load...]",
	Short:                 "Start loads (must be provisioned first)",
	Args:                  validateLoadArgs,
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, loadNames []string) {
		loads := cfg.GetLoads(loadNames...)
		client := clientPkg.NewClient(cfg)

		// Provisioning must be done separately with 'provision' command

		loaded := make(map[string]bool)
		for _, l := range loads {
			startChain := l.StartChain
			for _, l := range startChain {
				if _, exists := loaded[l.Name]; !exists {
					loaded[l.Name] = true
					log.Info("Starting load %s", color.CyanString(l.Name))
					if err := client.StartLoad(l); err != nil {
						log.Warn("Starting load failed: %s", err.Error())
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
		loads := cfg.GetLoads(loadNames...)
		client := clientPkg.NewClient(cfg)
		stopped := make(map[string]bool)

		for _, l := range loads {
			stopChain := l.StopChain
			for _, l := range stopChain {
				if _, exists := stopped[l.Name]; !exists {
					stopped[l.Name] = true
					log.Info("Stopping load %s", color.CyanString(l.Name))
					if err := client.StopLoad(l); err != nil {
						log.Warn("Stopping load failed: %s", err.Error())
					}
				}
			}
		}
	},
}

var killLoads = &cobra.Command{
	Use:                   "kill [--all | load...]",
	Short:                 "Kill loads",
	Args:                  validateLoadArgs,
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, loadNames []string) {
		loads := cfg.GetLoads(loadNames...)
		client := clientPkg.NewClient(cfg)
		killed := make(map[string]bool)

		for _, l := range loads {
			stopChain := l.StopChain
			for _, l := range stopChain {
				if _, exists := killed[l.Name]; !exists {
					killed[l.Name] = true
					log.Info("Killing load %s", color.CyanString(l.Name))
					if err := client.KillLoad(l); err != nil {
						log.Warn("Killing load failed: %s", err.Error())
					}
				}
			}
		}
	},
}

var deprovisionLoads = &cobra.Command{
	Use:                   "deprovision [--all | load...]",
	Short:                 "Remove provisioned (not started) loads",
	Args:                  validateLoadArgs,
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, loadNames []string) {
		loads := cfg.GetLoads(loadNames...)
		client := clientPkg.NewClient(cfg)
		deprovisioned := make(map[string]bool)

		for _, l := range loads {
			if _, exists := deprovisioned[l.Name]; !exists {
				deprovisioned[l.Name] = true
				log.Info("Deprovisioning load %s", color.CyanString(l.Name))
				if err := client.DeprovisionLoad(l); err != nil {
					log.Warn("Deprovisioning load failed: %s", err.Error())
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
		g := cfg.GetLoadsGraph()
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
		loads := cfg.GetLoads(loadNames...)
		if len(loads) == 0 {
			log.Fatal("Load not found")
		}
		if len(loads) > 1 {
			log.Fatal("Multiple loads found with that name")
		}

		load := loads[loadNames[0]]
		client := clientPkg.NewClient(cfg)
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
		loads := cfg.GetLoads(loadNames...)
		if len(loads) == 0 {
			log.Fatal("Load not found")
		}
		if len(loads) > 1 {
			log.Fatal("Multiple loads found with that name")
		}

		load := loads[loadNames[0]]
		client := clientPkg.NewClient(cfg)
		if err := client.ReadLoadStderr(load); err != nil {
			log.Fatal("Failed to read stderr: %s", err.Error())
		}
	},
}

func init() {
	startLoads.Flags().Bool("all", false, "All loads")
	provisionLoads.Flags().Bool("all", false, "All loads")
	listLoads.Flags().Bool("all", false, "All loads")
	stopLoads.Flags().Bool("all", false, "All loads")
	killLoads.Flags().Bool("all", false, "All loads")
	deprovisionLoads.Flags().Bool("all", false, "All loads")

	loadsCmd.AddCommand(graphLoads)
	loadsCmd.AddCommand(listLoads)
	loadsCmd.AddCommand(provisionLoads)
	loadsCmd.AddCommand(startLoads)
	loadsCmd.AddCommand(stdoutLoad)
	loadsCmd.AddCommand(stderrLoad)
	loadsCmd.AddCommand(stopLoads)
	loadsCmd.AddCommand(killLoads)
	loadsCmd.AddCommand(deprovisionLoads)
	rootCmd.AddCommand(loadsCmd)
}
