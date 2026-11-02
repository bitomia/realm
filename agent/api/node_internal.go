package api

import (
	"errors"

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
		if node.SameConfig(&current.Node) {
			return common.ErrNodeAlreadyConfigured
		}
		return common.ErrNodeConfigMismatch
	} else if errors.Is(err, common.ErrNodeNotConfigured) {
		return nil
	}
	return err
}
