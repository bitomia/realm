package api

import (
	"fmt"

	"github.com/bitomia/realm/common/dto"
)

func RunJob(request dto.JobRequest) (*dto.JobResult, error) {
	if request.Job == nil {
		return nil, fmt.Errorf("job nil")
	}

	value, err := request.Job.Driver.Run(request.Arguments...)
	if err != nil {
		err := err.Error()
		return &dto.JobResult{Value: nil, Err: &err}, nil
	}
	if value != nil {
		return &dto.JobResult{Value: value, Err: nil}, nil
	}

	return &dto.JobResult{}, nil
}
