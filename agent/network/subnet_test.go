package network

import (
	"encoding/json"

	"github.com/containernetworking/cni/libcni"

	"github.com/bitomia/realm/common/dto"
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

func ipamRoutes(t *testing.T, conf string) []string {
	t.Helper()
	confList, err := libcni.ConfListFromBytes([]byte(conf))
	assert.NoError(t, err)

	var bridge struct {
		IPAM struct {
			Routes []struct {
				Dst string `json:"dst"`
			} `json:"routes"`
		} `json:"ipam"`
	}
	assert.NoError(t, json.Unmarshal(confList.Plugins[0].Bytes, &bridge))

	var dsts []string
	for _, r := range bridge.IPAM.Routes {
		dsts = append(dsts, r.Dst)
	}
	return dsts
}

func TestCreateNetworkConfigRoutes(t *testing.T) {
	portmaps := []dto.Portmap{{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"}}

	// No mesh: everything but 10.0.0.0/8
	routes := ipamRoutes(t, createNetworkConfig("web", "10.0.1.0/24", true, portmaps, nil))
	assert.Len(t, routes, 8)
	assert.NotContains(t, routes, "10.100.0.0/24")

	// Mesh: its network is routed too, so containers can answer mesh peers
	routes = ipamRoutes(t, createNetworkConfig("web", "10.0.1.0/24", true, portmaps, []*net.IPNet{cidr(t, "10.100.0.0/24")}))
	assert.Len(t, routes, 9)
	assert.Contains(t, routes, "10.100.0.0/24")
	assert.Contains(t, routes, "128.0.0.0/1")
}
