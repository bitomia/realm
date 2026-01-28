package cpu

import (
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/process"

	"github.com/bitomia/realm/daemon/cruntime"

	"github.com/bitomia/realm/common/dto"
)

func GetNodeState() (*dto.NodeStateResponse, error) {
	ctx, client, err := cruntime.CreateClient()
	if err != nil {
		return nil, err
	}
	defer client.Close()

	cpuStat, cpuUsage, containersState, err := CollectNodeState(ctx, client)
	if err != nil {
		return nil, err
	}

	cpuCount, err := cpu.Counts(true)
	if err != nil {
		return nil, err
	}

	var nodeState dto.NodeStateResponse
	nodeState.Containers = containersState
	nodeState.NumCPU = cpuCount
	nodeState.UserCPU = uint64(cpuStat.User)
	nodeState.IdleCPU = uint64(cpuStat.Idle)
	nodeState.SystemCPU = uint64(cpuStat.System)
	nodeState.TotalCPU = uint64(cpuStat.User + cpuStat.System + cpuStat.Nice + cpuStat.Idle + cpuStat.Iowait + cpuStat.Irq + cpuStat.Softirq + cpuStat.Steal)
	nodeState.UsageCPUPercent = cpuUsage

	memStat, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}
	nodeState.TotalMem = memStat.Total
	nodeState.UsedMem = memStat.Used
	nodeState.FreeMem = memStat.Free
	nodeState.FreeMemPercent = float64(memStat.Available) / float64(memStat.Total) * 100
	nodeState.FreeStorage, _ = GetFreeStorage()

	// Get swap memory information
	swapStat, err := mem.SwapMemory()
	if err == nil {
		nodeState.SwapTotal = swapStat.Total
		nodeState.SwapUsed = swapStat.Used
		nodeState.SwapFree = swapStat.Free
	}

	// Get CPU load average (1-minute average)
	loadStat, err := load.Avg()
	if err == nil {
		// Normalize load by number of CPUs (load average from 0 to 1)
		nodeState.CpuLoadAvg = float32(loadStat.Load1 / float64(cpuCount))
	}

	// Get process counts
	processes, err := process.Processes()
	if err == nil {
		var procCounts struct {
			total    uint32
			sleeping uint32
			running  uint32
			zombie   uint32
			stopped  uint32
			idle     uint32
			threads  uint32
		}

		for _, proc := range processes {
			procCounts.total++

			// Get process status
			status, err := proc.Status()
			if err != nil || len(status) == 0 {
				continue
			}

			// Count by status (use first status string)
			switch status[0] {
			case "S":
				procCounts.sleeping++
			case "R":
				procCounts.running++
			case "Z":
				procCounts.zombie++
			case "T", "t":
				procCounts.stopped++
			case "I":
				procCounts.idle++
			}

			// Count threads
			numThreads, err := proc.NumThreads()
			if err == nil {
				procCounts.threads += uint32(numThreads)
			}
		}

		nodeState.ProcTotalCount = procCounts.total
		nodeState.ProcSleepingCount = procCounts.sleeping
		nodeState.ProcRunningCount = procCounts.running
		nodeState.ProcZombieCount = procCounts.zombie
		nodeState.ProcStoppedCount = procCounts.stopped
		nodeState.ProcIdleCount = procCounts.idle
		nodeState.ProcThreadsCount = procCounts.threads
	}

	return &nodeState, nil
}

// GetSystemInfo returns static system information about the host
func GetSystemInfo() (*dto.SystemInfo, error) {
	hostInfo := &dto.SystemInfo{}

	// Get host information
	info, err := host.Info()
	if err != nil {
		return nil, err
	}

	hostInfo.OsName = info.Platform + " " + info.PlatformVersion
	hostInfo.OsKind = getOsKind(info.OS)
	hostInfo.NetHostName = info.Hostname

	// Get CPU info
	cpuInfo, err := cpu.Info()
	if err != nil {
		return nil, err
	}
	if len(cpuInfo) > 0 {
		hostInfo.CpuModel = cpuInfo[0].ModelName
		hostInfo.CpuVendor = cpuInfo[0].VendorID
		hostInfo.CpuMhz = uint32(cpuInfo[0].Mhz)
		hostInfo.CpuTotalCores = uint32(cpuInfo[0].Cores)
	}

	// Get architecture
	hostInfo.OsArch = getCpuArch(runtime.GOARCH)

	// Get memory info
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}
	hostInfo.RamTotal = memInfo.Total

	// Get network information
	if err := populateNetworkInfo(hostInfo); err != nil {
		// Don't fail on network info errors, just continue
	}

	// Get boot time (approximate CPU start time)
	if info.BootTime > 0 {
		hostInfo.CpuStartTime = int64(info.BootTime) * 1e9 // Convert seconds to nanoseconds
	}

	// Calculate cores per socket and total sockets
	totalCores := hostInfo.CpuTotalCores
	if totalCores > 0 {
		// For simplicity, assume 1 socket unless we can determine otherwise
		hostInfo.CpuTotalSockets = 1
		hostInfo.CpuCoresPerSocket = totalCores
	}

	return hostInfo, nil
}

// getOsKind converts OS string to OsKind enum
func getOsKind(osName string) dto.OsKind {
	osName = strings.ToLower(osName)
	switch {
	case strings.Contains(osName, "windows"):
		return dto.WindowsOs
	case strings.Contains(osName, "android"):
		return dto.AndroidOs
	case strings.Contains(osName, "darwin") || strings.Contains(osName, "ios") || strings.Contains(osName, "tvos"):
		return dto.AppleOs
	case strings.Contains(osName, "linux"):
		return dto.LinuxOs
	case strings.Contains(osName, "unix"):
		return dto.UnixOs
	case strings.Contains(osName, "posix"):
		return dto.PosixOs
	default:
		return dto.OtherOs
	}
}

// getCpuArch converts GOARCH string to CpuArch enum
func getCpuArch(arch string) dto.CpuArch {
	switch arch {
	case "386":
		return dto.X86
	case "amd64":
		return dto.X64
	case "arm":
		return dto.Arm7 // Default ARM to ARM7
	case "arm64":
		return dto.Arm64
	case "mips", "mipsle", "mips64", "mips64le":
		return dto.Mips
	case "ppc64", "ppc64le":
		return dto.Ppc64
	case "sparc", "sparc64":
		return dto.Sparc
	default:
		return dto.OtherArch
	}
}

// populateNetworkInfo fills in network-related fields
func populateNetworkInfo(hostInfo *dto.SystemInfo) error {
	// Get primary interface and its details
	ifaces, err := net.Interfaces()
	if err != nil {
		return err
	}

	for _, iface := range ifaces {
		// Skip loopback and down interfaces
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					hostInfo.NetPrimaryIpAddr = ipnet.IP.String()
					hostInfo.NetMacAddress = iface.HardwareAddr.String()

					// Try to get default gateway (simplified approach)
					if gateway := getDefaultGateway(); gateway != "" {
						hostInfo.NetDefaultGateway = gateway
					}

					// Try to get DNS servers
					if dns := getDNSServers(); len(dns) > 0 {
						hostInfo.NetPrimaryDns = dns[0]
						if len(dns) > 1 {
							hostInfo.NetSecondaryDns = dns[1]
						}
					}

					return nil
				}
			}
		}
	}

	return nil
}

// getDefaultGateway attempts to get the default gateway
func getDefaultGateway() string {
	// Simple approach: read from /proc/net/route
	data, err := os.ReadFile("/proc/net/route")
	if err != nil {
		return ""
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines[1:] { // Skip header
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[1] == "00000000" { // Default route
			gatewayHex := fields[2]
			if gateway := hexToIP(gatewayHex); gateway != "" {
				return gateway
			}
		}
	}

	return ""
}

// getDNSServers attempts to read DNS servers from /etc/resolv.conf
func getDNSServers() []string {
	data, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		return nil
	}

	var dns []string
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "nameserver") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				dns = append(dns, parts[1])
			}
		}
	}

	return dns
}

// hexToIP converts hex string to IP address
func hexToIP(hexStr string) string {
	if len(hexStr) != 8 {
		return ""
	}

	var ip []string
	for i := 6; i >= 0; i -= 2 {
		hexByte := hexStr[i : i+2]
		if val, err := strconv.ParseInt(hexByte, 16, 0); err == nil {
			ip = append(ip, strconv.Itoa(int(val)))
		}
	}

	if len(ip) == 4 {
		return strings.Join(ip, ".")
	}

	return ""
}
