package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	clientPkg "github.com/bitomia/realm/clients/go"
	"github.com/bitomia/realm/cmd/log"
	"github.com/bitomia/realm/common"
)

func validateLoadArgs(cmd *cobra.Command, args []string) error {
	all, _ := cmd.Flags().GetBool("all")

	if all && len(args) > 0 {
		return fmt.Errorf("cannot use --all with load names")
	}

	if !all && len(args) == 0 {
		return fmt.Errorf("must specify --all or at least one load name")
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
		client.ProvisionLoads(loads, func(load *common.Load, err error) {
			if err == nil {
				log.Success("Provisioning %s completed", color.CyanString(load.Name))
			} else {
				log.Warn("Provisioning %s failed: %s", color.CyanString(load.Name), err.Error())
			}
		})
	},
}

var deprovisionLoads = &cobra.Command{
	Use:                   "deprovision [--all | load...]",
	Short:                 "Remove provisioned loads",
	Args:                  validateLoadArgs,
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, loadNames []string) {
		loads := cfg.GetLoads(loadNames...)
		client := clientPkg.NewClient(cfg)
		client.DeprovisionLoads(loads, func(load *common.Load, err error) {
			if err == nil {
				log.Success("Deprovisioning %s completed", color.CyanString(load.Name))
			} else {
				log.Warn("Deprovisioning %s failed: %s", color.CyanString(load.Name), err.Error())
			}
		})
	},
}

var deploymentsState = &cobra.Command{
	Use:                   "state [--all | load...]",
	Aliases:               []string{"ls"},
	Short:                 "Show state of deployments",
	Args:                  validateLoadArgs,
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, loadNames []string) {
		loads := cfg.GetLoads(loadNames...)
		if len(loads) == 0 {
			log.Error("No loads")
			return
		}
		client := clientPkg.NewClient(cfg)

		for _, state := range client.GetLoadsStates(loads) {
			var deploymentStatusStr string
			switch {
			case state.Err != nil:
				deploymentStatusStr = fmt.Sprintf("unknown %s", strings.TrimSpace(state.Err.Error()))
			case !state.Provisioned:
				deploymentStatusStr = "not provisioned"
			default:
				switch state.Status.StatusCode {
				case common.DeploymentStatusRunning:
					deploymentStatusStr = color.GreenString("running")
				case common.DeploymentStatusReady:
					deploymentStatusStr = color.HiBlueString("provisioned")
				case common.DeploymentStatusStopped:
					deploymentStatusStr = color.YellowString("stopped")
				case common.DeploymentStatusError:
					deploymentStatusStr = fmt.Sprintf("%s %s", color.RedString("error"), state.Status.Reason)
				default:
					deploymentStatusStr = string(state.Status.StatusCode)
				}
			}

			containerInfo := ""
			if len(state.ContainerNames) > 0 {
				containerInfo = fmt.Sprintf(" [%s]", strings.Join(state.ContainerNames, ", "))
			}
			load := state.Load
			color.White("%s (node %s) [%s]%s\n", color.CyanString(load.Name), color.YellowString(load.Node.Name), strings.TrimSpace(deploymentStatusStr), containerInfo)
			prettyJSON(load, "name", "node")
		}
	},
}

var startLoads = &cobra.Command{
	Use:                   "start [--all | load...]",
	Short:                 "Start loads",
	Args:                  validateLoadArgs,
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, loadNames []string) {
		loads := cfg.GetLoads(loadNames...)
		client := clientPkg.NewClient(cfg)
		client.StartLoads(loads, func(load *common.Load, err error) {
			if err == nil {
				log.Success("Starting %s completed", color.CyanString(load.Name))
			} else {
				log.Warn("Starting %s failed: %s", color.CyanString(load.Name), err.Error())
			}
		})
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
		client.StopLoads(loads, func(load *common.Load, err error) {
			if err == nil {
				log.Success("Stopping %s completed", color.CyanString(load.Name))
			} else {
				log.Warn("Stopping %s failed: %s", color.CyanString(load.Name), err.Error())
			}
		})
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
		client.KillLoads(loads, func(load *common.Load, err error) {
			if err != nil {
				log.Warn("Killing load %s failed: %s", color.CyanString(load.Name), err.Error())
			} else {
				log.Info("Load %s killed", color.CyanString(load.Name))
			}
		})
	},
}

var graphLoads = &cobra.Command{
	Use:                   "graph",
	Short:                 "Print the dependency graph",
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, loadNames []string) {
		client := clientPkg.NewClient(cfg)
		dependencies, err := client.LoadsDependencies()
		if err != nil {
			log.Fatal("%s", err.Error())
		}

		for _, d := range dependencies {
			if d.To == "" {
				fmt.Printf("%v\n", d.From)
			} else {
				fmt.Printf("%v -> %v\n", d.From, d.To)
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
		if err := client.StreamLoadStdout(load, os.Stdout); err != nil {
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
		if err := client.StreamLoadStderr(load, os.Stdout); err != nil {
			log.Fatal("Failed to read stderr: %s", err.Error())
		}
	},
}

func init() {
	startLoads.Flags().Bool("all", false, "All loads")
	provisionLoads.Flags().Bool("all", false, "All loads")
	deploymentsState.Flags().Bool("all", false, "All loads")
	stopLoads.Flags().Bool("all", false, "All loads")
	killLoads.Flags().Bool("all", false, "All loads")
	deprovisionLoads.Flags().Bool("all", false, "All loads")

	loadsCmd.AddCommand(graphLoads)
	loadsCmd.AddCommand(deploymentsState)
	loadsCmd.AddCommand(provisionLoads)
	loadsCmd.AddCommand(startLoads)
	loadsCmd.AddCommand(stdoutLoad)
	loadsCmd.AddCommand(stderrLoad)
	loadsCmd.AddCommand(stopLoads)
	loadsCmd.AddCommand(killLoads)
	loadsCmd.AddCommand(deprovisionLoads)
	rootCmd.AddCommand(loadsCmd)
}
