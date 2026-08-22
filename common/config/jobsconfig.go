package config

import (
	"fmt"
	"slices"

	"github.com/bitomia/realm/common"
)

var (
	jobsConfig map[string]*common.Job = make(map[string]*common.Job)
)

func ResetJobsConfig() {
	jobsConfig = make(map[string]*common.Job)
}

func newJobConfig(jobName string, node *common.Node, driver common.JobDriver) (*common.Job, error) {
	if _, exists := jobsConfig[jobName]; exists {
		return nil, fmt.Errorf("job '%s' not unique", jobName)
	}

	jobsConfig[jobName] = &common.Job{Name: jobName, Node: node, Driver: driver}
	return jobsConfig[jobName], nil
}

func GetJobs(jobsFilter ...string) map[string]*common.Job {
	jobs := make(map[string]*common.Job)
	for _, job := range jobsConfig {
		if len(jobsFilter) == 0 || slices.Contains(jobsFilter, job.Name) {
			jobs[job.Name] = job
		}
	}
	return jobs
}
