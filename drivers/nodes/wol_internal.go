package nodes

import (
	"fmt"
	"net"
)

func launchWakeOnLan(macAddress string) error {
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

	// Send the magic packet as UDP broadcast on port 9
	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{
		IP:   net.IPv4bcast,
		Port: 9,
	})
	if err != nil {
		return fmt.Errorf("failed to create udp connection: %w", err)
	}
	defer conn.Close()

	if _, err := conn.Write(packet); err != nil {
		return fmt.Errorf("failed to send wol packet: %w", err)
	}
	return nil
}
