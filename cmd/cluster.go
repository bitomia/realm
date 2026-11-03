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
		stateFile, _ := cmd.Flags().GetString("state")
		dry, _ := cmd.Flags().GetBool("dry")
		yes, _ := cmd.Flags().GetBool("yes")

		approve := func() (bool, error) {
			if yes {
				return true, nil
			}
			if !term.IsTerminal(int(os.Stdin.Fd())) {
				return false, errors.New("cannot ask for confirmation, stdin is not a terminal: use -y to update the state anyway")
			}
			return confirm(fmt.Sprintf("Update %s?", stateFile), os.Stdin, os.Stdout)
		}

		client := clientPkg.NewClient(cfg)
		if err := client.PlanCluster(stateFile, dry, approve); err != nil {
			log.Fatal("%s", err)
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
		stateFile, _ := cmd.Flags().GetString("state")

		client := clientPkg.NewClient(cfg)
		exitCode := client.StartCluster(cmd.Context(), stateFile)
		if exitCode == 0 {
			log.Success("Cluster started")
		}

		os.Exit(exitCode)
	},
}

var stopCmd = &cobra.Command{
	Use:     "stop [--state file]",
	GroupID: "main",
	Short:   "Stop loads and power-off cluster nodes",
	Long: "Stop loads and power-off cluster nodes.\n" +
		"Aborts powering off if any load fails to stop.\n" +
		"Aborts if the config differs from the state file.",
	DisableFlagsInUseLine: true,
	Args:                  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		stateFile, _ := cmd.Flags().GetString("state")

		client := clientPkg.NewClient(cfg)
		exitCode := client.StopCluster(cmd.Context(), stateFile)
		if exitCode == 0 {
			log.Success("Cluster stopped")
		}

		os.Exit(exitCode)
	},
}

var updateCmd = &cobra.Command{
	Use:                   "update [--state file]",
	GroupID:               "main",
	Short:                 "Roll out planned loads changes on a running cluster",
	DisableFlagsInUseLine: true,
	Args:                  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		stateFile, _ := cmd.Flags().GetString("state")

		client := clientPkg.NewClient(cfg)
		exitCode := client.UpdateCluster(cmd.Context(), stateFile)
		if exitCode == 0 {
			log.Success("Cluster updated")
		}

		os.Exit(exitCode)
	},
}

func init() {
	startCmd.Flags().String("state", common.DefaultClusterStateFile, "Cluster state file")
	rootCmd.AddCommand(startCmd)

	stopCmd.Flags().String("state", common.DefaultClusterStateFile, "Cluster state file")
	rootCmd.AddCommand(stopCmd)

	updateCmd.Flags().String("state", common.DefaultClusterStateFile, "Cluster state file")
	rootCmd.AddCommand(updateCmd)

	planCmd.Flags().String("state", common.DefaultClusterStateFile, "Cluster state file")
	planCmd.Flags().Bool("dry", false, "Show the differences without updating the state file")
	planCmd.Flags().BoolP("yes", "y", false, "Update the state file without asking for confirmation")
	planCmd.MarkFlagsMutuallyExclusive("dry", "yes")

	rootCmd.AddCommand(planCmd)
}
