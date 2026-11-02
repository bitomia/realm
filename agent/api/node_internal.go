package api

import (
	"errors"
	"reflect"

	"github.com/bitomia/realm/common"
)

func checkNodeConfig(repository common.NodesRepository, node *common.Node, guestMode bool) error {
	var current common.NodeEntry
	var err error
	if guestMode {
		current, err = repository.GetGuestNode(node.Name)
	} else {
		current, err = repository.GetSelf()
	}
	if err == nil {
		if sameNodeConfig(node, &current.Node) {
			return common.ErrNodeAlreadyConfigured
		}
		return common.ErrNodeConfigMismatch
	} else if errors.Is(err, common.ErrNodeNotConfigured) {
		return nil
	}
	return err
}

func sameNodeConfig(node *common.Node, current *common.Node) bool {
	return node.Name == current.Name &&
		node.Url == current.Url &&
		node.Timeout == current.Timeout &&
		reflect.DeepEqual(node.CloudInit, current.CloudInit) &&
		reflect.DeepEqual(node.Registries, current.Registries) &&
		node.Driver.Config().Equal(current.Driver.Config()) &&
		sameLoads(node.Loads, current.Loads)
}

// sameLoads tells whether both lists hold the same loads, regardless of their order
func sameLoads(loads []common.LoadConfig, current []common.LoadConfig) bool {
	if len(loads) != len(current) {
		return false
	}
	hashes := make(map[string]common.Hash, len(current))
	for _, load := range current {
		hashes[load.Name] = load.Hash()
	}
	for _, load := range loads {
		if hash, exists := hashes[load.Name]; !exists || hash != load.Hash() {
			return false
		}
	}
	return true
}
