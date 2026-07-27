//go:build !MESH

package mesh

import "github.com/bitomia/realm/common/config"

func Start(cfg *config.Config) error {
	return nil
}
