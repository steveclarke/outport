package allocator

import (
	"fmt"
	"hash/fnv"
)

const (
	MinPort         = 10000
	MaxPort         = 39999
	portRange       = MaxPort - MinPort + 1
	ReservedDNSPort = 15353
)

var reservedPorts = map[int]bool{
	ReservedDNSPort: true,
}

func HashPort(project, instance, service string) int {
	h := fnv.New32a()
	h.Write([]byte(fmt.Sprintf("%s/%s/%s", project, instance, service)))
	return int(h.Sum32()%uint32(portRange)) + MinPort
}

// Allocate assigns a port for a service. If isPortBusy is non-nil, it is called
// to check whether a candidate port is already in use on the system (e.g., by a
// non-Outport process). Busy ports are skipped via linear probing.
func Allocate(project, instance, service string, preferred int, usedPorts map[int]bool, isPortBusy func(int) bool) (int, error) {
	unavailable := func(p int) bool {
		if usedPorts[p] || reservedPorts[p] {
			return true
		}
		if isPortBusy != nil && isPortBusy(p) {
			return true
		}
		return false
	}

	if preferred > 0 && !unavailable(preferred) {
		return preferred, nil
	}

	start := HashPort(project, instance, service)
	port := start
	for unavailable(port) {
		port++
		if port > MaxPort {
			port = MinPort
		}
		if port == start {
			return 0, fmt.Errorf("no available ports in range %d-%d", MinPort, MaxPort)
		}
	}
	return port, nil
}
