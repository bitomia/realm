package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	clientPkg "github.com/bitomia/realm/clients/go"
	"github.com/bitomia/realm/cmd/log"
	"github.com/bitomia/realm/common"
)

var planCmd = &cobra.Command{
	Use:     "plan [--state file] [--dry | -y]",
	GroupID: "main",
	Short:   "Compute the cluster state from the config and save it to the state file",
	Long: "Compute and save the cluster state for the config.\n" +
		"The state is only stored once confirmed (or -y to force confirmation).\n" +
		"Use --dry to show differences without modifying the state file.",
	DisableFlagsInUseLine: true,
	Args:                  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		stateFile := getStateFile(cmd)
		dry, _ := cmd.Flags().GetBool("dry")
		yes, _ := cmd.Flags().GetBool("yes")

		approve := func(diffs []string) (bool, error) {
			common.PrintColoredDiff(diffs)

			if yes {
				return true, nil
			}
			if !term.IsTerminal(int(os.Stdin.Fd())) {
				return false, errors.New("cannot ask for confirmation, stdin is not a terminal")
			}

			if approved, err := confirm(fmt.Sprintf("Update %s?", stateFile), os.Stdin, os.Stdout); err != nil {
				return false, err
			} else {
				if !approved {
					log.Warn("Plan discarded, %s left untouched", stateFile)
				}
				return approved, nil
			}
		}

		if dry {
			diffs, err := clientPkg.DryPlanCluster(cfg, stateFile)
			if err != nil {
				log.Fatal("%s", err)
			}
			if len(diffs) == 0 {
				log.Success("Cluster state on %s is up to date", stateFile)
			} else {
				common.PrintColoredDiff(diffs)
			}
		} else {
			updated, err := clientPkg.PlanCluster(cfg, stateFile, approve)
			if err != nil {
				log.Fatal("%s", err)
			}
			if updated {
				log.Success("Cluster state planned on %s", stateFile)
			}
		}
	},
}

var startCmd = &cobra.Command{
	Use:     "start [--state file]",
	GroupID: "main",
	Short:   "Power-on, configure and start loads for cluster nodes,",
	Long: "Power-on, configure and start loads for cluster nodes.\n" +
		"Aborts if the config differs from the state file,\n" +
		"or if any running node holds a config different from the planned one.",
	DisableFlagsInUseLine: true,
	Args:                  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		client := newClient(cmd)
		if err := client.StartCluster(cmd.Context()); err != nil {
			log.Fatal("%s", err.Error())
		}
		log.Success("Cluster started")
	},
}

var stopCmd = &cobra.Command{
	Use:     "stop [--state file] [--message text] [--time offset] [-y]",
	GroupID: "main",
	Short:   "Stop loads and power-off cluster nodes",
	Long: "Stop loads and power-off cluster nodes.\n" +
		"Aborts powering off if any load fails to stop.\n" +
		"Aborts if the config differs from the state file.",
	DisableFlagsInUseLine: true,
	Args:                  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		wallMessage, _ := cmd.Flags().GetString("message")
		offsetTime, _ := cmd.Flags().GetUint32("time")
		yes, _ := cmd.Flags().GetBool("yes")

		if !yes {
			if !term.IsTerminal(int(os.Stdin.Fd())) {
				log.Fatal("cannot ask for confirmation, stdin is not a terminal")
			}
			ok, err := confirm("Stop the cluster?", os.Stdin, os.Stdout)
			if err != nil {
				log.Fatal("%s", err)
			}
			if !ok {
				return
			}
		}

		client := newClient(cmd)
		if err := client.StopCluster(cmd.Context(), wallMessage, offsetTime); err != nil {
			log.Fatal("%s", err.Error())
		}
		log.Success("Cluster stopped")
	},
}

var updateCmd = &cobra.Command{
	Use:                   "update [--state file]",
	GroupID:               "main",
	Short:                 "Roll out planned loads, timeout and registries changes on a running cluster",
	DisableFlagsInUseLine: true,
	Args:                  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		client := newClient(cmd)
		if err := client.UpdateCluster(cmd.Context()); err != nil {
			log.Fatal("%s", err.Error())
		}
		log.Success("Cluster updated")
	},
}

func init() {
	rootCmd.AddCommand(startCmd)

	stopCmd.Flags().String("message", "", "Wall message broadcast to logged-in users before shutdown")
	stopCmd.Flags().Uint32("time", 0, "Shutdown time offset, 0 shuts down now")
	stopCmd.Flags().BoolP("yes", "y", false, "Stop the cluster without asking for confirmation")
	rootCmd.AddCommand(stopCmd)

	rootCmd.AddCommand(updateCmd)

	planCmd.Flags().Bool("dry", false, "Show the differences without updating the state file")
	planCmd.Flags().BoolP("yes", "y", false, "Update the state file without asking for confirmation")
	planCmd.MarkFlagsMutuallyExclusive("dry", "yes")

	rootCmd.AddCommand(planCmd)
}
