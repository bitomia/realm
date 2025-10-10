package config

import (
	"crypto/sha256"
	"encoding/json"
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
	name      string
	driver    LoadDriver
	dependsOn []*Load
	node      *Node
}

func (n *Load) Hash() [32]byte {
	data, err := json.Marshal(*n)
	if err != nil {
		panic(err)
	}
	return sha256.Sum256(data)
}
