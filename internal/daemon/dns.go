package daemon

import (
	"net"
	"strings"

	"github.com/miekg/dns"
)

// loopback is the IPv4 address 127.0.0.1, used as the answer for all *.test
// DNS queries. All local services are accessible on localhost, so every .test
// hostname resolves to this same address. The actual routing to the correct
// service port happens in the HTTP proxy layer via the Host header, not at
// the DNS level.
var loopback = net.IPv4(127, 0, 0, 1).To4()

// NewDNSServer creates a UDP DNS server that answers A-record queries for any
// hostname ending in ".test" with 127.0.0.1. All other query types and domains
// receive an NXDOMAIN (name error) response. The server listens on the given
// addr (typically "127.0.0.1:15353") and includes the given ttl in seconds on
// every response record. On macOS, a resolver file at /etc/resolver/test
// directs the OS to send all *.test queries to this server.
func NewDNSServer(addr string, ttl uint32) *dns.Server {
	handler := dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Authoritative = true

		for _, q := range r.Question {
			if q.Qtype == dns.TypeA && strings.HasSuffix(q.Name, ".test.") {
				m.Answer = append(m.Answer, &dns.A{
					Hdr: dns.RR_Header{
						Name:   q.Name,
						Rrtype: dns.TypeA,
						Class:  dns.ClassINET,
						Ttl:    ttl,
					},
					A: loopback,
				})
			}
		}

		if len(m.Answer) == 0 {
			m.Rcode = dns.RcodeNameError
		}

		_ = w.WriteMsg(m)
	})

	return &dns.Server{
		Addr:    addr,
		Net:     "udp",
		Handler: handler,
	}
}
