package dto

import "github.com/bitomia/realm/common"

type JobRequest struct {
	Job       *common.Job `json:"job"`
	Arguments []string    `json:"arguments,omitempty"`
}

type JobResult struct {
	Value *string `json:"value,omitempty"`
	Err   *string `json:"err,omitempty"`
}
