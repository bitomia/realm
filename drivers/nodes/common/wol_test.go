package common

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBroadcastAddr(t *testing.T) {
	tests := []struct {
		cidr     string
		expected net.IP
	}{
		{"193.100.0.5/24", net.ParseIP("193.100.0.255").To4()},
		{"10.1.2.3/8", net.ParseIP("10.255.255.255").To4()},
		{"172.16.5.4/20", net.ParseIP("172.16.15.255").To4()},
		{"192.168.1.10/31", nil},
		{"192.168.1.10/32", nil},
		{"192.168.240.101/0", net.IPv4bcast.To4()},
		{"fe80::1/64", nil},
	}
	for _, tt := range tests {
		t.Run(tt.cidr, func(t *testing.T) {
			ip, ipNet, err := net.ParseCIDR(tt.cidr)
			assert.NoError(t, err)
			ipNet.IP = ip
			assert.Equal(t, tt.expected, broadcastAddr(ipNet))
		})
	}
}

func TestBroadcastTargets(t *testing.T) {
	targets, err := broadcastTargets()
	assert.NoError(t, err)
	for _, tt := range targets {
		assert.NotNil(t, tt.bcast.To4())
		assert.NotNil(t, tt.local.To4())
		assert.False(t, tt.local.IsLoopback())
	}
}

func TestLaunchWakeOnLan(t *testing.T) {
	assert.Error(t, LaunchWakeOnLan("not-a-mac"))
	assert.NoError(t, LaunchWakeOnLan("3c:ec:ef:e3:c8:67"))
}
