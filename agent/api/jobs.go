package api

import (
	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/common/dto"
)

func RunJob(w common.JobResultWriter, request dto.JobRequest) error {
	driver, err := common.BuildJobDriver(request.JobDriverConfig)
	if err != nil {
		return err
	}
	if err := driver.Run(w, request.Arguments...); err != nil {
		return w.WriteError(err)
	}
	return nil
}
