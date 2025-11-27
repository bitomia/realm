package config

import (
	"crypto/sha256"
	"fmt"

	"github.com/bitomia/realm/drivers/loads"

	"github.com/bitomia/realm/internal"
)

var (
	loadsRepository map[string]*loads.Load = make(map[string]*loads.Load)
)

func newLoad(name string, node *internal.Node, driver loads.LoadDriver) (*loads.Load, error) {
	if _, exists := loadsRepository[name]; exists {
		return nil, fmt.Errorf("Node name not unique")
	}
	loadsRepository[name] = &loads.Load{Name: name, Node: node, Driver: driver}
	return loadsRepository[name], nil
}

func GetLoads() map[string]*loads.Load {
	loads := make(map[string]*loads.Load)
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
