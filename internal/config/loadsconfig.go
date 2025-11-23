package config

import (
	"crypto/sha256"
	"fmt"

	internalLoads "github.com/bitomia/realm/internal/loads"
	"github.com/bitomia/realm/internal/node"
)

var (
	loadsRepository map[string]*internalLoads.Load = make(map[string]*internalLoads.Load)
)

func newLoad(name string, node *node.Node, driver internalLoads.LoadDriver) (*internalLoads.Load, error) {
	if _, exists := loadsRepository[name]; exists {
		return nil, fmt.Errorf("Node name not unique")
	}
	loadsRepository[name] = &internalLoads.Load{Name: name, Node: node, Driver: driver}
	return loadsRepository[name], nil
}

func GetLoads() map[string]*internalLoads.Load {
	loads := make(map[string]*internalLoads.Load)
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
