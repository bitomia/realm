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
		switch {
		case !node.SameNodeStaticConfig(&current.Node):
			return common.ErrNodeConfigMismatch
		case !node.SameLoadsConfig(&current.Node):
			return common.ErrNodeLoadsMismatch
		case !node.SameNodeLiveConfig(&current.Node):
			return common.ErrNodeLiveConfigMismatch
		default:
			return common.ErrNodeAlreadyConfigured
		}
	} else if errors.Is(err, common.ErrNodeNotConfigured) {
		return nil
	}
	return err
}
