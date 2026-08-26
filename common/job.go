package common

import (
	"crypto/sha256"
	"encoding/json"
	"net/http"
)

type Job struct {
	Name   string
	Node   *Node
	Driver JobDriver
}

type JobConfig struct {
	Name         string      `json:"name"`
	Node         string      `json:"node"`
	Driver       JobDriverID `json:"driver"`
	DriverConfig any         `json:"driver_config"`
}

func (j *Job) MarshalJSON() ([]byte, error) {
	return json.Marshal(&JobConfig{
		Name:         j.Name,
		Node:         j.Node.Name,
		Driver:       j.Driver.ID(),
		DriverConfig: j.Driver.Config().DriverConfig,
	})
}

func (j *Job) UnmarshalJSON(data []byte) error {
	aux := JobConfig{}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	driver, err := BuildJobDriver(JobDriverConfig{Driver: aux.Driver, DriverConfig: aux.DriverConfig})
	if err != nil {
		return err
	}
	j.Name = aux.Name
	j.Driver = driver

	return nil
}

func (j *Job) Hash() [32]byte {
	data, err := json.Marshal(JobConfig{
		Name:         j.Name,
		Node:         j.Node.Name,
		Driver:       j.Driver.ID(),
		DriverConfig: j.Driver.Config().DriverConfig,
	})
	if err != nil {
		panic(err)
	}

	return sha256.Sum256(data)
}

type JobResult struct {
	Value *string `json:"value,omitempty"`
	Err   *string `json:"err,omitempty"`
}

type JobResultWriter struct {
	enc     *json.Encoder
	flusher http.Flusher
}

func NewJobResultWriter(w http.ResponseWriter) JobResultWriter {
	w.Header().Set("Content-Type", "application/x-ndjson")
	flusher, _ := w.(http.Flusher)
	return JobResultWriter{enc: json.NewEncoder(w), flusher: flusher}
}

func (j *JobResultWriter) Write(r JobResult) error {
	if err := j.enc.Encode(r); err != nil {
		return err
	}
	if j.flusher != nil {
		j.flusher.Flush()
	}
	return nil
}

func (j *JobResultWriter) WriteValue(value string) error {
	return j.Write(JobResult{Value: &value})
}

func (j *JobResultWriter) WriteError(err error) error {
	msg := err.Error()
	return j.Write(JobResult{Err: &msg})
}
