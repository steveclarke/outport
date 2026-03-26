// Package lanip detects the machine's LAN IPv4 address. This is used by the
// "outport share" command to display the local network URL and QR code so that
// mobile devices on the same Wi-Fi network can access locally running services.
//
// Auto-detection follows a two-phase strategy: first it checks well-known
// interface names (en0, en1 on macOS; eth0, wlan0 on Linux), then falls back
// to scanning all interfaces while
// filtering out virtual, loopback, and point-to-point interfaces. A specific
// interface name can also be provided to skip auto-detection entirely.
package lanip

import (
	"fmt"
	"net"
	"strings"
)

var virtualPrefixes = []string{
	"utun", "bridge", "veth", "docker", "vmnet", "lo",
}

var preferredNames = []string{"en0", "en1", "eth0", "wlan0"}

// Detect returns the LAN IPv4 address for the given interface name. If
// interfaceName is empty, it auto-detects a suitable LAN interface by first
// trying well-known names (en0, en1) and then scanning all non-virtual, non-
// loopback interfaces for one with an IPv4 address. Returns an error if no
// suitable interface is found, with a hint to use the --interface flag.
func Detect(interfaceName string) (net.IP, error) {
	if interfaceName != "" {
		return fromInterface(interfaceName)
	}
	return autoDetect()
}

func fromInterface(name string) (net.IP, error) {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return nil, fmt.Errorf("interface %q: %w", name, err)
	}
	return ipv4FromInterface(iface)
}

func autoDetect() (net.IP, error) {
	for _, name := range preferredNames {
		iface, err := net.InterfaceByName(name)
		if err != nil {
			continue
		}
		if ip, err := ipv4FromInterface(iface); err == nil {
			return ip, nil
		}
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("listing interfaces: %w", err)
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if iface.Flags&net.FlagPointToPoint != 0 {
			continue
		}
		if isVirtual(iface.Name) {
			continue
		}
		if ip, err := ipv4FromInterface(&iface); err == nil {
			return ip, nil
		}
	}
	return nil, fmt.Errorf("no suitable LAN interface found. Use --interface to specify one")
}

func ipv4FromInterface(iface *net.Interface) (net.IP, error) {
	if iface.Flags&net.FlagUp == 0 {
		return nil, fmt.Errorf("interface %s is down", iface.Name)
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return nil, err
	}
	for _, addr := range addrs {
		var ip net.IP
		switch v := addr.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}
		if ip == nil || ip.IsLoopback() {
			continue
		}
		if ipv4 := ip.To4(); ipv4 != nil {
			return ipv4, nil
		}
	}
	return nil, fmt.Errorf("no IPv4 address on %s", iface.Name)
}

func isVirtual(name string) bool {
	lower := strings.ToLower(name)
	for _, prefix := range virtualPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}
