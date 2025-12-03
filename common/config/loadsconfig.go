package config

import (
	"crypto/sha256"
	"fmt"

	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/internal"
)

var (
	loadsRepository map[string]*common.Load = make(map[string]*common.Load)
)

func newLoad(name string, node *internal.Node, driver common.LoadDriver) (*common.Load, error) {
	if _, exists := loadsRepository[name]; exists {
		return nil, fmt.Errorf("Node name not unique")
	}
	loadsRepository[name] = &common.Load{Name: name, Node: node, Driver: driver}
	return loadsRepository[name], nil
}

func GetLoads() map[string]*common.Load {
	loads := make(map[string]*common.Load)
	for _, load := range loadsRepository {
		loads[load.Name] = load
	}
	return loads
}

func GetLoadsHash() [32]byte {
	var hashes [][32]byte
	for _, l := range loadsRepository {
		hashes = append(hashes, l.Hash())
	}

	var combined []byte
	for _, h := range hashes {
		combined = append(combined, h[:]...)
	}

	return sha256.Sum256(combined)
}
