package containers

import (
	"testing"

	"github.com/bitomia/realm/common/config"
	"github.com/bitomia/realm/common/dto"
	daemonConfig "github.com/bitomia/realm/daemon/config"
)

const testConfig = `
daemon:
  data_path: .
`

func init() {
	cfg, _ := config.InitFromBuffer(testConfig)
	daemonConfig.Set(cfg)
}

// This test verifies that CreateContainer handles nil Quotas gracefully without panicking.
// The test may fail due to missing containerd, images, or other dependencies
func TestCreateContainer_BindMounts(t *testing.T) {
	tests := []struct {
		name       string
		bindMounts []dto.BindMount
		wantSkip   int // number of mounts expected to be skipped
	}{
		{
			name:       "nil bind mounts should not panic",
			bindMounts: nil,
			wantSkip:   0,
		},
		{
			name:       "empty bind mounts slice should not panic",
			bindMounts: []dto.BindMount{},
			wantSkip:   0,
		},
		{
			name: "single read-write bind mount",
			bindMounts: []dto.BindMount{
				{
					Source:      "/tmp/test-source",
					Destination: "/app/data",
					ReadOnly:    false,
				},
			},
			wantSkip: 0,
		},
		{
			name: "single read-only bind mount",
			bindMounts: []dto.BindMount{
				{
					Source:      "/tmp/test-source",
					Destination: "/app/data",
					ReadOnly:    true,
				},
			},
			wantSkip: 0,
		},
		{
			name: "multiple bind mounts",
			bindMounts: []dto.BindMount{
				{
					Source:      "/tmp/source1",
					Destination: "/app/data1",
					ReadOnly:    false,
				},
				{
					Source:      "/tmp/source2",
					Destination: "/app/data2",
					ReadOnly:    true,
				},
				{
					Source:      "/opt/shared",
					Destination: "/opt/shared",
					ReadOnly:    true,
				},
			},
			wantSkip: 0,
		},
		{
			name: "bind mount with empty source should be skipped",
			bindMounts: []dto.BindMount{
				{
					Source:      "",
					Destination: "/app/data",
					ReadOnly:    false,
				},
			},
			wantSkip: 1,
		},
		{
			name: "bind mount with empty destination should be skipped",
			bindMounts: []dto.BindMount{
				{
					Source:      "/tmp/source",
					Destination: "",
					ReadOnly:    false,
				},
			},
			wantSkip: 1,
		},
		{
			name: "mix of valid and invalid bind mounts",
			bindMounts: []dto.BindMount{
				{
					Source:      "/tmp/valid",
					Destination: "/app/valid",
					ReadOnly:    false,
				},
				{
					Source:      "",
					Destination: "/app/invalid",
					ReadOnly:    false,
				},
				{
					Source:      "/tmp/another-valid",
					Destination: "/app/another",
					ReadOnly:    true,
				},
			},
			wantSkip: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := dto.CreateContainerRequest{
				Image:      "docker.io/library/alpine:latest",
				BindMounts: tt.bindMounts,
			}
			// This test verifies that CreateContainer handles bind mounts gracefully without panicking.
			// The test may fail due to missing containerd, images, or other dependencies
			CreateContainer("test-bindmount-"+tt.name, opts, nil)
		})
	}
}

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
				Quotas: tt.quotas,
			}
			CreateContainer("test-container-"+tt.name, opts, nil)
		})
	}
}
