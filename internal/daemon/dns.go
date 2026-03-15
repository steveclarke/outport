package daemon

import (
	"net"
	"strings"

	"github.com/miekg/dns"
)

const dnsTTL = 60 // seconds

// NewDNSServer creates a DNS server that resolves *.test to 127.0.0.1.
func NewDNSServer(addr string) *dns.Server {
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
						Ttl:    dnsTTL,
					},
					A: net.ParseIP("127.0.0.1"),
				})
			}
		}

		if len(m.Answer) == 0 {
			m.Rcode = dns.RcodeNameError
		}

		w.WriteMsg(m)
	})

	return &dns.Server{
		Addr:    addr,
		Net:     "udp",
		Handler: handler,
	}
}
