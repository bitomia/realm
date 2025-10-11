package config

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
)

type LoadDriverType int

const (
	ContainerDriverType LoadDriverType = iota
	ProcessDriverType
)

var loadDriver = map[LoadDriverType]string{
	ContainerDriverType: "container",
	ProcessDriverType:   "process",
}

func (d LoadDriverType) String() string {
	return loadDriver[d]
}

type LoadDriver interface {
	GetDriverType() LoadDriverType
}

type Load struct {
	Name      string
	Driver    LoadDriver
	DependsOn []*Load
	Node      *Node
}

func (l *Load) MarshalJSON() ([]byte, error) {
	var nodeName string
	if l.Node != nil {
		nodeName = l.Node.Name
	}
	dependsOn := make([]string, len(l.DependsOn))
	for i, dep := range l.DependsOn {
		dependsOn[i] = dep.Name
	}
	return json.Marshal(&struct {
		Name      string     `json:"name"`
		Driver    LoadDriver `json:"driver"`
		DependsOn []string   `json:"depends_on"`
		Node      string     `json:"node"`
	}{
		Name:      l.Name,
		Driver:    l.Driver,
		DependsOn: dependsOn,
		Node:      nodeName,
	})
}

func (l *Load) Hash() [32]byte {
	data, err := json.Marshal(l)
	fmt.Printf("%s\n", data)
	if err != nil {
		panic(err)
	}
	return sha256.Sum256(data)
}
