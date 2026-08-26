package dto

import (
	"github.com/bitomia/realm/common"
)

type JobRequest struct {
	Name string `json:"name"`
	common.JobDriverConfig
	Arguments []string `json:"arguments,omitempty"`
}
