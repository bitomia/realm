package cmd

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"

	"github.com/fatih/color"
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

var startCmd = &cobra.Command{
	Use:     "start [--state file]",
	GroupID: "main",
	Short:   "Power-on and configure the cluster nodes",
	Long: "Power-on and configure the cluster nodes.\n" +
		"Aborts if the config differs from the state file,\n" +
		"or if any running node holds a config different from the planned one.",
	DisableFlagsInUseLine: true,
	Args:                  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		stateFile, _ := cmd.Flags().GetString("state")

		if err := checkClusterState(stateFile); err != nil {
			log.Fatal("%s", err)
		}

		client := clientPkg.NewClient(cfg)
		startSeq := startNodesSequence(cmd.Context(), &client, cfg.GetNodes())

		if isSuccessStartSequence(startSeq) {
			log.Success("Nodes successfully started")
			return
		}

		log.Error("Nodes drifted")
		for _, name := range slices.Sorted(maps.Keys(startSeq)) {
			if stage := startSeq[name]; stage == nodeStageConfigured {
				log.Success(" %s: %s", color.CyanString(name), stage)
			} else {
				log.Warn(" %s: %s (expected %s)", color.CyanString(name), stage, nodeStageConfigured)
			}
		}

		os.Exit(1)
	},
}

func checkClusterState(stateFile string) error {
	previous, err := common.ReadClusterStateFile(stateFile)
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("no cluster state found on %s: run 'plan' first", stateFile)
	} else if err != nil {
		return err
	}

	planned := cfg.GetClusterState()
	diff := common.DiffClusterStates(previous, planned)
	if diff.IsEmpty() {
		return nil
	}

	if err := common.PrintClusterStateDiff(diff, previous, planned); err != nil {
		return err
	}
	return fmt.Errorf("config differs from cluster state on %s: run 'plan' to review and update it", stateFile)
}

func init() {
	startCmd.Flags().String("state", common.DefaultClusterStateFile, "Cluster state file")
	rootCmd.AddCommand(startCmd)

	planCmd.Flags().String("state", common.DefaultClusterStateFile, "Cluster state file")
	planCmd.Flags().Bool("dry", false, "Show the differences without updating the state file")
	planCmd.Flags().BoolP("yes", "y", false, "Update the state file without asking for confirmation")
	planCmd.MarkFlagsMutuallyExclusive("dry", "yes")

	rootCmd.AddCommand(planCmd)
}
