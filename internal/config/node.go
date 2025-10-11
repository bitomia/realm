package config

import (
	"crypto/sha256"
	"encoding/json"
)

// Physical node inside the cluster
type Node struct {
	Name string `mapstructure:"name"`
	Url  string `mapstructure:"url"`
}

func (n *Node) Hash() [32]byte {
	data, err := json.Marshal(*n)
	if err != nil {
		panic(err)
	}
	return sha256.Sum256(data)
}
