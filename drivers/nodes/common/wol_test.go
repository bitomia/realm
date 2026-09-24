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

func TestBroadcastAddrs(t *testing.T) {
	addrs, err := broadcastAddrs()
	assert.NoError(t, err)
	for _, ip := range addrs {
		assert.NotNil(t, ip.To4())
		assert.False(t, ip.IsLoopback())
	}
}

func TestLaunchWakeOnLan(t *testing.T) {
	assert.Error(t, LaunchWakeOnLan("not-a-mac"))
	assert.NoError(t, LaunchWakeOnLan("3c:ec:ef:e3:c8:67"))
}
