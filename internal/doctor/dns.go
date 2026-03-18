package doctor

import (
	"fmt"
	"net"
	"time"

	"github.com/miekg/dns"
)

// checkDNSResolving sends a UDP query to the given resolver address for
// "outport-check.test" and verifies it returns 127.0.0.1.
func checkDNSResolving(resolverAddr string) *Result {
	name := "DNS resolving *.test"

	c := new(dns.Client)
	c.Timeout = 2 * time.Second

	m := new(dns.Msg)
	m.SetQuestion("outport-check.test.", dns.TypeA)

	r, _, err := c.Exchange(m, resolverAddr)
	if err != nil {
		return &Result{
			Name:    name,
			Status:  Fail,
			Message: fmt.Sprintf("DNS query failed: %v", err),
			Fix:     "Run: outport system restart",
		}
	}

	if len(r.Answer) == 0 {
		return &Result{Name: name, Status: Fail, Message: "DNS query returned no answers", Fix: "Run: outport system restart"}
	}

	for _, ans := range r.Answer {
		if a, ok := ans.(*dns.A); ok {
			if a.A.Equal(net.IPv4(127, 0, 0, 1)) {
				return &Result{Name: name, Status: Pass, Message: "DNS resolving *.test → 127.0.0.1"}
			}
		}
	}

	return &Result{Name: name, Status: Fail, Message: "DNS query did not return 127.0.0.1", Fix: "Run: outport system restart"}
}
