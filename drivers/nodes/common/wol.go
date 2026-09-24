package common

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
)

const wolPort = 9

// Broadcast wake-on-lan on all capable interfaces
func LaunchWakeOnLan(macAddress string) error {
	mac, err := net.ParseMAC(macAddress)
	if err != nil {
		return fmt.Errorf("invalid mac address: %w", err)
	}

	// Build the magic packet: 6 bytes of 0xFF followed by 16 repetitions of the MAC address
	packet := make([]byte, 102)
	for i := range 6 {
		packet[i] = 0xFF
	}
	for i := range 16 {
		copy(packet[6+i*6:], mac)
	}

	targets, err := broadcastAddrs()
	if err != nil {
		return fmt.Errorf("failed to list network interfaces: %w", err)
	}
	// Use IPv4bcast if there is no available interfaces
	if len(targets) == 0 {
		targets = []net.IP{net.IPv4bcast}
	}

	var errs []error
	for _, ip := range targets {
		if err := sendPacket(packet, ip, wolPort); err != nil {
			errs = append(errs, err)
			continue
		}
		slog.Debug("wol packet sent", "mac", mac.String(), "broadcast", ip.String())
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to send wol packet: %w", errors.Join(errs...))
	}
	return nil
}

func sendPacket(packet []byte, ip net.IP, port int) error {
	conn, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: ip, Port: port})
	if err != nil {
		return fmt.Errorf("%s: %w", ip, err)
	}
	defer conn.Close()

	if _, err := conn.Write(packet); err != nil {
		return fmt.Errorf("%s: %w", ip, err)
	}
	return nil
}

// broadcastAddrs returns the IPv4 directed broadcast address of every
// interface that broadcast capable and not a loopback
func broadcastAddrs() ([]net.IP, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	var result []net.IP
	seen := map[string]bool{}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagRunning == 0 || iface.Flags&net.FlagBroadcast == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip := broadcastAddr(ipNet)
			if ip == nil || seen[ip.String()] {
				continue
			}
			seen[ip.String()] = true
			result = append(result, ip)
		}
	}
	return result, nil
}

// broadcastAddr returns the directed broadcast address of an IPv4 network
func broadcastAddr(ipNet *net.IPNet) net.IP {
	ip := ipNet.IP.To4()
	if ip == nil {
		return nil
	}
	mask := ipNet.Mask
	if len(mask) == net.IPv6len {
		mask = mask[12:]
	}
	if ones, bits := mask.Size(); bits != 32 || ones >= 31 {
		return nil
	}

	bcast := make(net.IP, net.IPv4len)
	for i := range bcast {
		bcast[i] = ip[i] | ^mask[i]
	}
	return bcast
}
