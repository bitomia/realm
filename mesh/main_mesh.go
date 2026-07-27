//go:build MESH

package mesh

import "github.com/bitomia/realm/common/config"

func Start(cfg *config.Config) error {
	if cfg != nil && cfg.MeshConfig != nil {
		if err := StartMesh(cfg); err != nil {
			return err
		}
	}
	return nil
}
