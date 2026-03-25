// Package allocator assigns deterministic port numbers to services using content-based
// hashing. It is the core of Outport's "port orchestration" — given the same project,
// instance, and service name, it always produces the same port number. This makes
// `outport up` idempotent: re-running it yields identical allocations.
//
// The algorithm hashes the string "{project}/{instance}/{service}" with FNV-32a and
// maps the result into the port range 10000-39999. When collisions occur (two services
// hash to the same port), linear probing finds the next available port, wrapping around
// from 39999 back to 10000 if needed.
//
// Port 15353 is permanently reserved for the Outport daemon's DNS server and is never
// assigned to a service.
package allocator

import (
	"fmt"
	"hash/fnv"
)

const (
	// MinPort is the lowest port number that can be assigned to a service.
	// Ports below 10000 are avoided to stay clear of well-known system ports
	// and common development server defaults (3000, 5000, 8080, etc.).
	MinPort = 10000

	// MaxPort is the highest port number that can be assigned to a service.
	// The range 10000-39999 provides 30,000 possible ports, which is more than
	// enough for typical local development while staying well below the
	// ephemeral port range (49152-65535).
	MaxPort = 39999

	portRange = MaxPort - MinPort + 1

	// ReservedDNSPort is the port used by the Outport daemon's DNS server.
	// It listens on 15353 (rather than the standard DNS port 53, which requires
	// root privileges). macOS resolver files in /etc/resolver/ direct .test
	// domain lookups to this port. This port is excluded from service allocation.
	ReservedDNSPort = 15353
)

var reservedPorts = map[int]bool{
	ReservedDNSPort: true,
}

// HashPort computes a deterministic port number for a service by hashing the
// composite key "{project}/{instance}/{service}" with the FNV-32a algorithm.
// The hash is mapped into the allocatable range [MinPort, MaxPort] using modular
// arithmetic. The same inputs always produce the same output, which is what makes
// Outport's port assignments stable across runs.
//
// Note: this function does not check for collisions or reserved ports. Use
// Allocate for collision-safe assignment.
func HashPort(project, instance, service string) int {
	h := fnv.New32a()
	h.Write([]byte(fmt.Sprintf("%s/%s/%s", project, instance, service)))
	return int(h.Sum32()%uint32(portRange)) + MinPort
}

// Allocate assigns a port for a service, handling collisions and reservations.
// It first checks if a preferred port (from the outport.yml config) is available
// and returns it immediately if so. Otherwise, it computes a deterministic starting
// port via HashPort and uses linear probing to find the next available port.
//
// A port is considered unavailable if any of these are true:
//   - It appears in usedPorts (already allocated to another service in this run).
//   - It is a reserved port (e.g., the daemon DNS port 15353).
//   - The isPortBusy callback returns true (the port is in use by a non-Outport
//     process on the system). This callback may be nil, in which case system-level
//     checks are skipped.
//
// Linear probing increments the port by 1 on each collision, wrapping from MaxPort
// back to MinPort. If every port in the range is exhausted (extremely unlikely with
// 30,000 ports), an error is returned.
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
