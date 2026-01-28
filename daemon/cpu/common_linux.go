//go:build linux
// +build linux

package cpu

import (
	"bufio"
	"os"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

func GetMemLimit() float64 {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return float64(^uint64(0))
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "MemTotal:") {
			fields := strings.Fields(scanner.Text())
			if len(fields) >= 2 {
				memKb, err := strconv.ParseUint(fields[1], 10, 64)
				if err == nil {
					return float64(memKb * 1024) // kB to bytes
				}
			}
			break
		}
	}
	return float64(^uint64(0))
}

func GetFreeStorage() (uint64, error) {
	var fsStat unix.Statfs_t
	if err := unix.Statfs("/", &fsStat); err != nil {
		return 0, err
	}
	return fsStat.Bfree * uint64(fsStat.Bsize), nil
}
