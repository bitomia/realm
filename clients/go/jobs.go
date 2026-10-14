package golang

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/common/dto"
)

func (c *Client) RunJob(job *common.Job, handle func(common.JobResult), arguments ...string) error {
	request := dto.JobRequest{
		Name:            job.Name,
		JobDriverConfig: job.Driver.Config(),
		Arguments:       arguments,
	}

	payload := new(bytes.Buffer)
	if err := json.NewEncoder(payload).Encode(request); err != nil {
		return err
	}

	resp, err := c.doStreamRequest("POST", job.Node.Url, "/jobs", payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	decoder := json.NewDecoder(resp.Body)
	for {
		var result common.JobResult
		if err := decoder.Decode(&result); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("failed to parse job result: %v", err)
		}
		handle(result)
	}
}
