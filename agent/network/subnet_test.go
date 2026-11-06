package network

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func cidr(t *testing.T, s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	assert.NoError(t, err)
	return n
}

func TestSubnetFromOffset(t *testing.T) {
	assert.Equal(t, "10.0.1.0/24", subnetFromOffset(1).String())
	assert.Equal(t, "10.100.0.0/24", subnetFromOffset(100*256).String())
}

func TestSubnetsOverlap(t *testing.T) {
	web := cidr(t, "10.0.1.0/24")
	// Hetzner private network route and a node's /32 address
	assert.True(t, subnetsOverlap(web, cidr(t, "10.0.0.0/16")))
	assert.True(t, subnetsOverlap(web, &net.IPNet{IP: net.ParseIP("10.0.1.11"), Mask: net.CIDRMask(32, 32)}))
	// Address with host bits set, as netlink returns them
	assert.True(t, subnetsOverlap(web, &net.IPNet{IP: net.ParseIP("10.0.1.1"), Mask: net.CIDRMask(24, 32)}))
	assert.False(t, subnetsOverlap(web, cidr(t, "10.100.0.0/24")))
	assert.False(t, subnetsOverlap(web, cidr(t, "172.16.0.0/16")))
}
