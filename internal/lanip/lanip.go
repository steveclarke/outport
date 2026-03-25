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

// Detect returns the LAN IPv4 address for the given interface name.
// If interfaceName is empty, it auto-detects a suitable LAN interface.
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
