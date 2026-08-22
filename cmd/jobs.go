package main

import (
	"fmt"

	"github.com/spf13/cobra"

	clientPkg "github.com/bitomia/realm/cmd/client"
	"github.com/bitomia/realm/cmd/log"
)

var jobsCmd = &cobra.Command{
	Use:     "jobs",
	Aliases: []string{"j"},
	Short:   "Interface with jobs",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Realm CLI. Use -h for help.")
	},
}

var startJob = &cobra.Command{
	Use:                   "run [job]",
	Short:                 "Run a job",
	DisableFlagsInUseLine: true,
	SilenceUsage:          true,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("job not specified")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		job := cfg.GetJob(args[0])
		if job == nil {
			return fmt.Errorf("job %s not found", args[0])
		}

		client := clientPkg.NewClient(cfg)

		if ret, err := client.RunJob(job, args[1:]...); err != nil {
			log.Warn("Error running job: %s", err.Error())
		} else {
			if ret.Err != nil {
				log.Info("Job failed: %s", *ret.Err)
			} else if ret.Value != nil {
				log.Info("Job succeeded: %s", *ret.Value)
			} else {
				log.Info("Job succeeded")
			}
		}

		return nil
	},
}

func init() {
	jobsCmd.AddCommand(startJob)
	rootCmd.AddCommand(jobsCmd)
}
