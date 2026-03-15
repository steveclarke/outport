package daemon

import (
	"fmt"
	"net"
	"testing"

	"github.com/miekg/dns"
)

// startTestDNS starts a DNS server on a random UDP port and returns its
// address. The server is shut down when the test finishes.
func startTestDNS(t *testing.T) string {
	t.Helper()

	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	addr := pc.LocalAddr().String()

	srv := NewDNSServer(addr)
	srv.PacketConn = pc

	started := make(chan struct{})
	srv.NotifyStartedFunc = func() { close(started) }

	go func() {
		if err := srv.ActivateAndServe(); err != nil {
			// Server.Shutdown causes ActivateAndServe to return; ignore that.
		}
	}()

	<-started

	t.Cleanup(func() { srv.Shutdown() })

	return addr
}

func TestDNSServerResolvesTestDomain(t *testing.T) {
	addr := startTestDNS(t)

	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn("foo.test"), dns.TypeA)

	r, _, err := new(dns.Client).Exchange(m, addr)
	if err != nil {
		t.Fatalf("exchange: %v", err)
	}

	if r.Rcode != dns.RcodeSuccess {
		t.Fatalf("expected NOERROR, got %s", dns.RcodeToString[r.Rcode])
	}

	if len(r.Answer) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(r.Answer))
	}

	a, ok := r.Answer[0].(*dns.A)
	if !ok {
		t.Fatalf("expected A record, got %T", r.Answer[0])
	}

	if got := a.A.String(); got != "127.0.0.1" {
		t.Fatalf("expected 127.0.0.1, got %s", got)
	}

	if a.Hdr.Ttl != dnsTTL {
		t.Fatalf("expected TTL %d, got %d", dnsTTL, a.Hdr.Ttl)
	}
}

func TestDNSServerResolvesSubdomain(t *testing.T) {
	addr := startTestDNS(t)

	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn("portal.unio.test"), dns.TypeA)

	r, _, err := new(dns.Client).Exchange(m, addr)
	if err != nil {
		t.Fatalf("exchange: %v", err)
	}

	if r.Rcode != dns.RcodeSuccess {
		t.Fatalf("expected NOERROR, got %s", dns.RcodeToString[r.Rcode])
	}

	if len(r.Answer) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(r.Answer))
	}

	a, ok := r.Answer[0].(*dns.A)
	if !ok {
		t.Fatalf("expected A record, got %T", r.Answer[0])
	}

	if got := a.A.String(); got != "127.0.0.1" {
		t.Fatalf("expected 127.0.0.1, got %s", got)
	}

	// Verify the answer name matches the queried name.
	if a.Hdr.Name != "portal.unio.test." {
		t.Fatalf("expected name portal.unio.test., got %s", a.Hdr.Name)
	}
}

func TestDNSServerRejectsNonTestDomain(t *testing.T) {
	addr := startTestDNS(t)

	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn("foo.com"), dns.TypeA)

	r, _, err := new(dns.Client).Exchange(m, addr)
	if err != nil {
		t.Fatalf("exchange: %v", err)
	}

	if r.Rcode != dns.RcodeNameError {
		t.Fatalf("expected NXDOMAIN, got %s", dns.RcodeToString[r.Rcode])
	}

	if len(r.Answer) != 0 {
		t.Fatalf("expected no answers, got %d", len(r.Answer))
	}
}

func TestDNSServerIgnoresNonAQueries(t *testing.T) {
	addr := startTestDNS(t)

	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn("foo.test"), dns.TypeAAAA)

	r, _, err := new(dns.Client).Exchange(m, addr)
	if err != nil {
		t.Fatalf("exchange: %v", err)
	}

	if r.Rcode != dns.RcodeNameError {
		t.Fatalf("expected NXDOMAIN for AAAA query, got %s", dns.RcodeToString[r.Rcode])
	}

	if len(r.Answer) != 0 {
		t.Fatalf("expected no answers, got %d", len(r.Answer))
	}
}

func TestNewDNSServerReturnsConfiguredServer(t *testing.T) {
	addr := "127.0.0.1:15353"
	srv := NewDNSServer(addr)

	if srv.Addr != addr {
		t.Fatalf("expected addr %s, got %s", addr, srv.Addr)
	}

	if srv.Net != "udp" {
		t.Fatalf("expected net udp, got %s", srv.Net)
	}

	if srv.Handler == nil {
		t.Fatal("expected handler to be set")
	}

	_ = fmt.Sprintf("server: %v", srv) // ensure no panic on inspect
}
