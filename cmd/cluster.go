package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/bitomia/realm/cmd/log"
	"github.com/bitomia/realm/common"
)

var planCmd = &cobra.Command{
	Use:   "plan [--state file] [--dry | -y]",
	Short: "Compute the cluster state from the config and save it to the state file",
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

		if err := plan(stateFile, dry, approve); err != nil {
			log.Fatal("%s", err)
		}
	},
}

func plan(stateFile string, dry bool, approve func() (bool, error)) error {
	previous, err := common.ReadClusterStateFile(stateFile)
	if errors.Is(err, os.ErrNotExist) {
		previous = nil
	} else if err != nil {
		return err
	}

	planned := cfg.GetClusterState()
	diff := common.DiffClusterStates(previous, planned)
	if diff.IsEmpty() {
		log.Success("No changes, cluster state on %s is up to date", stateFile)
		return nil
	}

	if err := common.PrintClusterStateDiff(diff, previous, planned); err != nil {
		return err
	}

	if dry {
		log.Info("Dry run, %s left untouched", stateFile)
		return nil
	}

	approved, err := approve()
	if err != nil {
		return err
	}
	if !approved {
		log.Warn("Plan discarded, %s left untouched", stateFile)
		return nil
	}

	if err := planned.WriteFile(stateFile); err != nil {
		return err
	}
	log.Success("Cluster state planned on %s", stateFile)
	return nil
}

func init() {
	planCmd.Flags().String("state", common.DefaultClusterStateFile, "Cluster state file")
	planCmd.Flags().Bool("dry", false, "Show the differences without updating the state file")
	planCmd.Flags().BoolP("yes", "y", false, "Update the state file without asking for confirmation")
	planCmd.MarkFlagsMutuallyExclusive("dry", "yes")

	rootCmd.AddCommand(planCmd)
}
