package common

import (
	"crypto/sha256"
	"encoding/json"
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
