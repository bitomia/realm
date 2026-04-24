package internal

import (
	"fmt"
	"net"
	"strings"
	"time"

	"golang.org/x/net/ipv4"
)

const (
	mdnsAddr             = "224.0.0.251:5353"
	defaultQueryTimeout  = 1000 * time.Millisecond
	defaultReadTimeout   = 500 * time.Millisecond
	defaultDetailTimeout = 100 * time.Millisecond
)

type QueryConfig struct {
	QueryTimeout  time.Duration // Total time to spend querying
	ReadTimeout   time.Duration // Timeout for individual read operations
	DetailTimeout time.Duration // Timeout for service detail queries
}

type DNSHeader struct {
	ID      uint16
	Flags   uint16
	QDCount uint16
	ANCount uint16
	NSCount uint16
	ARCount uint16
}

type DNSQuestion struct {
	Name  string
	Type  uint16
	Class uint16
}

type ServiceInfo struct {
	Name     string
	Hostname string
	Port     uint16
	Priority uint16
	Weight   uint16
	IPs      []string
	TXT      []string
}

func QueryServices(serviceName string) (map[string]*ServiceInfo, error) {
	return QueryServicesWithConfig(serviceName, QueryConfig{
		QueryTimeout:  defaultQueryTimeout,
		ReadTimeout:   defaultReadTimeout,
		DetailTimeout: defaultDetailTimeout,
	})
}

// serviceName example: "_services._dns-sd._udp.local"
func QueryServicesWithConfig(serviceName string, config QueryConfig) (map[string]*ServiceInfo, error) {
	addr, err := net.ResolveUDPAddr("udp", mdnsAddr)
	if err != nil {
		return nil, fmt.Errorf("error resolving mDNS address: %v", err)
	}

	conn, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   net.IPv4zero,
		Port: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating UDP listener: %v", err)
	}
	defer conn.Close()

	p := ipv4.NewPacketConn(conn)
	defer p.Close()

	err = p.SetMulticastTTL(64)
	if err != nil {
		return nil, fmt.Errorf("error setting multicast TTL: %v", err)
	}

	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("error getting interfaces: %v", err)
	}

	multicastAddr := &net.UDPAddr{IP: net.IPv4(224, 0, 0, 251)}
	joinedCount := 0
	var joinErrors []string

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagMulticast == 0 {
			continue
		}

		err = p.JoinGroup(&iface, multicastAddr)
		if err != nil {
			joinErrors = append(joinErrors, fmt.Sprintf("interface %s: %v", iface.Name, err))
		} else {
			joinedCount++
		}
	}

	if joinedCount == 0 {
		if len(joinErrors) > 0 {
			return nil, fmt.Errorf("failed to join multicast group on any interface: %s", strings.Join(joinErrors, "; "))
		}
		return nil, fmt.Errorf("no suitable interfaces found for multicast communication")
	}

	query := buildMDNSQuery(serviceName)

	_, err = p.WriteTo(query, nil, addr)
	if err != nil {
		return nil, fmt.Errorf("error sending query: %v", err)
	}

	endTime := time.Now().Add(config.QueryTimeout)
	responseCount := 0
	services := make(map[string]*ServiceInfo)
	hostnames := make(map[string]bool)

	for time.Now().Before(endTime) {
		_ = conn.SetReadDeadline(time.Now().Add(config.ReadTimeout))

		buffer := make([]byte, 1500)
		n, _, err := conn.ReadFromUDP(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			fmt.Printf("Error reading response: %v\n", err)
			continue
		}
		responseCount++

		_ = parseMDNSResponse(buffer[:n], services, hostnames)
	}

	for serviceName := range services {
		queryServiceDetails(p, addr, serviceName)
		time.Sleep(config.DetailTimeout)
	}

	time.Sleep(config.DetailTimeout)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(config.DetailTimeout))
		buffer := make([]byte, 1500)
		n, _, err := conn.ReadFromUDP(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				break
			}
			continue
		}
		_ = parseMDNSResponse(buffer[:n], services, hostnames)
	}

	return services, nil
}

func buildMDNSQuery(service string) []byte {
	query := make([]byte, 0, 512)

	query = append(query, 0x00, 0x00)
	query = append(query, 0x00, 0x00)
	query = append(query, 0x00, 0x01)
	query = append(query, 0x00, 0x00)
	query = append(query, 0x00, 0x00)
	query = append(query, 0x00, 0x00)

	query = append(query, encodeDNSName(service)...)

	query = append(query, 0x00, 0x0C)
	query = append(query, 0x00, 0x01)

	return query
}

func encodeDNSName(name string) []byte {
	encoded := make([]byte, 0, len(name)+2)

	labels := splitDNSName(name)
	for _, label := range labels {
		encoded = append(encoded, byte(len(label)))
		encoded = append(encoded, []byte(label)...)
	}
	encoded = append(encoded, 0x00)

	return encoded
}

func splitDNSName(name string) []string {
	labels := make([]string, 0)
	current := ""

	for _, char := range name {
		if char == '.' {
			if current != "" {
				labels = append(labels, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}

	if current != "" {
		labels = append(labels, current)
	}

	return labels
}

func parseMDNSResponse(data []byte, services map[string]*ServiceInfo, hostnames map[string]bool) error {
	if len(data) < 12 {
		return fmt.Errorf("response too short")
	}

	header := parseDNSHeader(data[:12])
	offset := 12
	for i := 0; i < int(header.ANCount); i++ {
		offset = parseResourceRecord(data, offset, "Answer", services, hostnames)
		if offset == -1 {
			break
		}
	}
	return nil
}

func parseDNSHeader(data []byte) DNSHeader {
	return DNSHeader{
		ID:      uint16(data[0])<<8 | uint16(data[1]),
		Flags:   uint16(data[2])<<8 | uint16(data[3]),
		QDCount: uint16(data[4])<<8 | uint16(data[5]),
		ANCount: uint16(data[6])<<8 | uint16(data[7]),
		NSCount: uint16(data[8])<<8 | uint16(data[9]),
		ARCount: uint16(data[10])<<8 | uint16(data[11]),
	}
}

func parseDNSName(data []byte, offset int) (string, int) {
	name := ""
	originalOffset := offset
	jumped := false

	for offset < len(data) {
		length := data[offset]

		if length == 0 {
			offset++
			break
		}

		if length&0xC0 == 0xC0 {
			if !jumped {
				originalOffset = offset + 2
				jumped = true
			}
			offset = int(uint16(length&0x3F)<<8 | uint16(data[offset+1]))
			continue
		}

		offset++
		if offset+int(length) > len(data) {
			break
		}

		if name != "" {
			name += "."
		}
		name += string(data[offset : offset+int(length)])
		offset += int(length)
	}

	if jumped {
		return name, originalOffset
	}
	return name, offset
}

func parseResourceRecord(data []byte, offset int, recordType string, services map[string]*ServiceInfo, hostnames map[string]bool) int {
	if offset >= len(data) {
		return -1
	}

	name, newOffset := parseDNSName(data, offset)
	if newOffset+10 > len(data) {
		return -1
	}

	rtype := uint16(data[newOffset])<<8 | uint16(data[newOffset+1])
	rdlen := uint16(data[newOffset+8])<<8 | uint16(data[newOffset+9])

	dataStart := newOffset + 10
	dataEnd := dataStart + int(rdlen)

	if dataEnd > len(data) {
		return -1
	}

	recordData := data[dataStart:dataEnd]
	switch rtype {
	case 1: // A record
		if len(recordData) == 4 {
			ip := fmt.Sprintf("%d.%d.%d.%d", recordData[0], recordData[1], recordData[2], recordData[3])

			for _, service := range services {
				if service.Hostname == name {
					service.IPs = append(service.IPs, ip)
				}
			}
		}
	case 12: // PTR record
		ptrName, _ := parseDNSName(data, dataStart)

		if strings.HasSuffix(name, "._tcp.local") || strings.HasSuffix(name, "._udp.local") {
			if services[ptrName] == nil {
				services[ptrName] = &ServiceInfo{Name: ptrName}
			}
		}
	case 16: // TXT record
		txtData := parseTXTRecord(recordData)

		if service, exists := services[name]; exists {
			service.TXT = append(service.TXT, txtData)
		}
	case 28: // AAAA record
		if len(recordData) == 16 {
			ip := fmt.Sprintf("%x:%x:%x:%x:%x:%x:%x:%x",
				uint16(recordData[0])<<8|uint16(recordData[1]),
				uint16(recordData[2])<<8|uint16(recordData[3]),
				uint16(recordData[4])<<8|uint16(recordData[5]),
				uint16(recordData[6])<<8|uint16(recordData[7]),
				uint16(recordData[8])<<8|uint16(recordData[9]),
				uint16(recordData[10])<<8|uint16(recordData[11]),
				uint16(recordData[12])<<8|uint16(recordData[13]),
				uint16(recordData[14])<<8|uint16(recordData[15]))

			for _, service := range services {
				if service.Hostname == name {
					service.IPs = append(service.IPs, ip)
				}
			}
		}
	case 33: // SRV record
		if len(recordData) >= 6 {
			priority := uint16(recordData[0])<<8 | uint16(recordData[1])
			weight := uint16(recordData[2])<<8 | uint16(recordData[3])
			port := uint16(recordData[4])<<8 | uint16(recordData[5])
			target, _ := parseDNSName(data, dataStart+6)

			if service, exists := services[name]; exists {
				service.Hostname = target
				service.Port = port
				service.Priority = priority
				service.Weight = weight
				hostnames[target] = true
			}
		}
	}

	return dataEnd
}

func parseTXTRecord(data []byte) string {
	result := ""
	offset := 0

	for offset < len(data) {
		if offset >= len(data) {
			break
		}

		length := int(data[offset])
		offset++

		if offset+length > len(data) {
			break
		}

		if result != "" {
			result += "; "
		}

		result += string(data[offset : offset+length])
		offset += length
	}

	return result
}

func queryServiceDetails(p *ipv4.PacketConn, addr *net.UDPAddr, serviceName string) {
	srvQuery := buildMDNSQuery(serviceName)
	_, _ = p.WriteTo(srvQuery, nil, addr)
}
