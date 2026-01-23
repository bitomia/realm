package internal

import (
	"log/slog"
	"net"
)

const bytesToMB = 1024.0 * 1024.0

func ToMB(bytes float64) float64 {
	return bytes / bytesToMB
}

func AutodetectIPAddress() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err == nil {
		defer conn.Close()
		localAddr := conn.LocalAddr().(*net.UDPAddr)
		return localAddr.IP.String()
	}

	// Fallback: iterate through interfaces
	interfaces, err := net.Interfaces()
	if err != nil {
		slog.Warn("Failed to get network interfaces", "error", err)
		return ""
	}

	for _, iface := range interfaces {
		// Skip interfaces that are down or loopback
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			// Return the first non-loopback IPv4 address
			if ip != nil && ip.To4() != nil && !ip.IsLoopback() {
				return ip.String()
			}
		}
	}

	return ""
}
