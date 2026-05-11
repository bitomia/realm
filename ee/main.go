//go:build !EE

package ee

import "github.com/bitomia/realm/common/config"

func Start(cfg *config.Config) error {
	return nil
}
