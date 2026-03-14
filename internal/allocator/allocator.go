package allocator

import (
	"fmt"
	"hash/fnv"
)

const (
	MinPort   = 10000
	MaxPort   = 39999
	portRange = MaxPort - MinPort + 1
)

func HashPort(project, instance, service string) int {
	h := fnv.New32a()
	h.Write([]byte(fmt.Sprintf("%s/%s/%s", project, instance, service)))
	return int(h.Sum32()%uint32(portRange)) + MinPort
}

func Allocate(project, instance, service string, preferred int, usedPorts map[int]bool) (int, error) {
	if preferred > 0 && !usedPorts[preferred] {
		return preferred, nil
	}
	start := HashPort(project, instance, service)
	port := start
	for usedPorts[port] {
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
