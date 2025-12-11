package containers

import (
	"testing"

	"github.com/bitomia/realm/common/config"
	"github.com/bitomia/realm/common/dto"
)

const testConfig = `
daemon:
  id_path: ./realm.id

nodes:
  test:
    url: http://localhost:9000
`

func init() {
	config.InitFromBuffer(testConfig)
}

// This test verifies that CreateContainer handles nil Quotas gracefully without panicking.
// The test may fail due to missing containerd, images, or other dependencies
func TestCreateContainer_NilQuotasRegression(t *testing.T) {
	tests := []struct {
		name   string
		quotas *dto.Quotas
	}{
		{
			name:   "nil quotas should not panic",
			quotas: nil,
		},
		{
			name: "quotas with all nil fields should not panic",
			quotas: &dto.Quotas{
				MemLimit:  nil,
				CpuShares: nil,
				CpuCFS:    nil,
			},
		},
		{
			name: "quotas with memory limit only",
			quotas: &dto.Quotas{
				MemLimit:  func() *uint64 { v := uint64(512); return &v }(),
				CpuShares: nil,
				CpuCFS:    nil,
			},
		},
		{
			name: "quotas with cpu shares only",
			quotas: &dto.Quotas{
				MemLimit:  nil,
				CpuShares: func() *uint64 { v := uint64(1024); return &v }(),
				CpuCFS:    nil,
			},
		},
		{
			name: "quotas with cpucfs only",
			quotas: &dto.Quotas{
				MemLimit:  nil,
				CpuShares: nil,
				CpuCFS: &dto.CpuCFS{
					CpuQuota:  100000,
					CpuPeriod: 100000,
				},
			},
		},
		{
			name: "quotas with all fields set",
			quotas: &dto.Quotas{
				MemLimit:  func() *uint64 { v := uint64(512); return &v }(),
				CpuShares: func() *uint64 { v := uint64(1024); return &v }(),
				CpuCFS: &dto.CpuCFS{
					CpuQuota:  100000,
					CpuPeriod: 100000,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := dto.CreateContainerRequest{
				Image:  "docker.io/library/alpine:latest",
				Owner:  "test-user",
				Quotas: tt.quotas,
			}
			CreateContainer("test-container-"+tt.name, opts, nil)
		})
	}
}
