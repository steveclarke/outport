# `outport doctor` Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a top-level `outport doctor` diagnostic command that checks the health of all Outport infrastructure and reports pass/warn/fail with actionable fix suggestions.

**Architecture:** New `internal/doctor/` package with `Check`, `Result`, `Runner` types. System checks always run; project checks run when `.outport.yml` is found. A few exports are added to `internal/platform/` and `internal/certmanager/` so doctor can reuse existing logic without duplication.

**Tech Stack:** Go, Cobra CLI, `miekg/dns` (already a dependency), `portcheck`, `platform`, `certmanager`, `config`, `registry`

**Spec:** `docs/superpowers/specs/2026-03-18-outport-doctor-design.md`

---

### Task 1: `internal/doctor/` — Core types and Runner

**Files:**
- Create: `internal/doctor/doctor.go`
- Create: `internal/doctor/doctor_test.go`

- [ ] **Step 1: Write failing tests for Runner**

```go
// internal/doctor/doctor_test.go
package doctor

import "testing"

func TestRunnerAllPass(t *testing.T) {
	r := &Runner{}
	r.Add(Check{
		Name:     "check1",
		Category: "Test",
		Run: func() *Result {
			return &Result{Name: "check1", Status: Pass, Message: "ok"}
		},
	})
	r.Add(Check{
		Name:     "check2",
		Category: "Test",
		Run: func() *Result {
			return &Result{Name: "check2", Status: Pass, Message: "ok"}
		},
	})
	results := r.Run()
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if HasFailures(results) {
		t.Error("expected no failures")
	}
}

func TestRunnerWithWarn(t *testing.T) {
	r := &Runner{}
	r.Add(Check{
		Name:     "warn-check",
		Category: "Test",
		Run: func() *Result {
			return &Result{Name: "warn-check", Status: Warn, Message: "warning", Fix: "do something"}
		},
	})
	results := r.Run()
	if results[0].Status != Warn {
		t.Errorf("expected Warn, got %v", results[0].Status)
	}
	if HasFailures(results) {
		t.Error("warnings should not count as failures")
	}
}

func TestRunnerWithFail(t *testing.T) {
	r := &Runner{}
	r.Add(Check{
		Name:     "fail-check",
		Category: "Test",
		Run: func() *Result {
			return &Result{Name: "fail-check", Status: Fail, Message: "broken", Fix: "fix it"}
		},
	})
	results := r.Run()
	if results[0].Status != Fail {
		t.Errorf("expected Fail, got %v", results[0].Status)
	}
	if !HasFailures(results) {
		t.Error("expected failures")
	}
}

func TestRunnerMixed(t *testing.T) {
	r := &Runner{}
	r.Add(Check{Name: "a", Category: "Cat1", Run: func() *Result {
		return &Result{Name: "a", Status: Pass, Message: "ok"}
	}})
	r.Add(Check{Name: "b", Category: "Cat1", Run: func() *Result {
		return &Result{Name: "b", Status: Warn, Message: "meh", Fix: "try this"}
	}})
	r.Add(Check{Name: "c", Category: "Cat2", Run: func() *Result {
		return &Result{Name: "c", Status: Fail, Message: "bad", Fix: "fix it"}
	}})
	results := r.Run()
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	// Verify ordering matches insertion order
	if results[0].Name != "a" || results[1].Name != "b" || results[2].Name != "c" {
		t.Error("results should preserve insertion order")
	}
	if !HasFailures(results) {
		t.Error("expected failures due to fail check")
	}
}

func TestRunnerEmpty(t *testing.T) {
	r := &Runner{}
	results := r.Run()
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
	if HasFailures(results) {
		t.Error("empty results should not have failures")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/doctor/ -v`
Expected: FAIL — package does not exist yet

- [ ] **Step 3: Write the implementation**

```go
// internal/doctor/doctor.go
package doctor

// Status represents the outcome of a health check.
type Status int

const (
	Pass Status = iota
	Warn
	Fail
)

// String returns the lowercase status label for JSON output.
func (s Status) String() string {
	switch s {
	case Pass:
		return "pass"
	case Warn:
		return "warn"
	case Fail:
		return "fail"
	default:
		return "unknown"
	}
}

// Check is a single diagnostic check.
type Check struct {
	Name     string
	Category string
	Run      func() *Result
}

// Result is the outcome of running a Check.
type Result struct {
	Name     string
	Category string
	Status   Status
	Message  string
	Fix      string
}

// Runner collects and executes checks sequentially.
type Runner struct {
	checks []Check
}

// Add appends a check to the runner.
func (r *Runner) Add(c Check) {
	r.checks = append(r.checks, c)
}

// Run executes all checks in order and returns the results.
func (r *Runner) Run() []Result {
	results := make([]Result, 0, len(r.checks))
	for _, c := range r.checks {
		res := c.Run()
		res.Category = c.Category
		results = append(results, *res)
	}
	return results
}

// HasFailures returns true if any result has Fail status.
func HasFailures(results []Result) bool {
	for _, r := range results {
		if r.Status == Fail {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/doctor/ -v`
Expected: PASS (all 5 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/doctor/doctor.go internal/doctor/doctor_test.go
git commit -m "feat(doctor): add core types and Runner"
```

---

### Task 2: Export platform helpers for doctor

**Files:**
- Modify: `internal/platform/darwin.go`

This task exports `PlistPath()`, `ResolverPath`, `ResolverContent`, and adds `IsCATrusted()`. Also updates `WriteResolverFile()` to use the new constant.

- [ ] **Step 1: Write failing test for IsCATrusted**

No unit test for `IsCATrusted` — it shells out to `security verify-cert` which requires macOS system state. We'll verify it works via manual `outport doctor` invocation during integration testing. The function is a thin wrapper.

- [ ] **Step 2: Add exports and IsCATrusted**

In `internal/platform/darwin.go`:

1. Export the resolver content as a constant and use it in `WriteResolverFile`:
```go
const (
	ResolverPath    = "/etc/resolver/test"
	ResolverContent = "nameserver 127.0.0.1\nport 15353\n"
	plistName       = "dev.outport.daemon.plist"
	plistLabel      = "dev.outport.daemon"
)
```

2. Rename `resolverPath` usages to `ResolverPath` throughout the file.

3. Update `WriteResolverFile` to use `ResolverContent` instead of the local `content` variable.

4. Export `plistPath` as `PlistPath`:
```go
// PlistPath returns the path to the LaunchAgent plist file.
func PlistPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Library", "LaunchAgents", plistName)
}
```

5. Update all internal callers of `plistPath()` to `PlistPath()`.

6. Add `IsCATrusted`:
```go
// IsCATrusted checks if the CA certificate is trusted in the system keychain
// by running "security verify-cert".
func IsCATrusted(certPath string) bool {
	err := exec.Command("security", "verify-cert", "-c", certPath).Run()
	return err == nil
}
```

- [ ] **Step 3: Run full test suite to verify no regressions**

Run: `go test ./... -count=1`
Expected: PASS

- [ ] **Step 4: Run lint**

Run: `golangci-lint run`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/platform/darwin.go
git commit -m "refactor(platform): export PlistPath, ResolverPath, ResolverContent; add IsCATrusted"
```

---

### Task 3: System checks — file and process checks

**Files:**
- Create: `internal/doctor/checks.go`
- Create: `internal/doctor/checks_test.go`

These are the checks that don't require network I/O: resolver file (1-2), plist (3-4), agent loaded (5), CA files (9-10), CA expiry (11), CA trusted (12), registry (13), cloudflared (14).

- [ ] **Step 1: Write failing tests for testable check logic**

```go
// internal/doctor/checks_test.go
package doctor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckResolverContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test")

	// Missing file
	res := checkResolverContent(path, "nameserver 127.0.0.1\nport 15353\n")
	if res.Status != Fail {
		t.Errorf("expected Fail for missing file, got %v", res.Status)
	}

	// Wrong content
	os.WriteFile(path, []byte("wrong"), 0644)
	res = checkResolverContent(path, "nameserver 127.0.0.1\nport 15353\n")
	if res.Status != Fail {
		t.Errorf("expected Fail for wrong content, got %v", res.Status)
	}

	// Correct content
	os.WriteFile(path, []byte("nameserver 127.0.0.1\nport 15353\n"), 0644)
	res = checkResolverContent(path, "nameserver 127.0.0.1\nport 15353\n")
	if res.Status != Pass {
		t.Errorf("expected Pass, got %v", res.Status)
	}
}

func TestCheckFileExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "exists")

	// Missing
	res := checkFileExists(path, "test file", "fix it")
	if res.Status != Fail {
		t.Errorf("expected Fail, got %v", res.Status)
	}

	// Exists
	os.WriteFile(path, []byte("x"), 0644)
	res = checkFileExists(path, "test file", "fix it")
	if res.Status != Pass {
		t.Errorf("expected Pass, got %v", res.Status)
	}
}

func TestCheckPlistBinary(t *testing.T) {
	dir := t.TempDir()
	plistPath := filepath.Join(dir, "test.plist")
	binaryPath := filepath.Join(dir, "outport")

	// Use a realistic plist matching the actual structure from platform.GeneratePlist —
	// includes Label, RunAtLoad, KeepAlive, Sockets, and StandardOut/ErrorPath keys
	// to ensure the XML parser correctly finds ProgramArguments among other keys.
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>dev.outport.daemon</string>
    <key>ProgramArguments</key>
    <array>
        <string>` + binaryPath + `</string>
        <string>daemon</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>Sockets</key>
    <dict>
        <key>HTTPSocket</key>
        <dict>
            <key>SockNodeName</key>
            <string>127.0.0.1</string>
            <key>SockServiceName</key>
            <string>80</string>
        </dict>
    </dict>
    <key>StandardOutPath</key>
    <string>/tmp/outport-daemon.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/outport-daemon.log</string>
</dict>
</plist>`
	os.WriteFile(plistPath, []byte(plist), 0644)
	os.WriteFile(binaryPath, []byte("binary"), 0755)

	res := checkPlistBinary(plistPath)
	if res.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", res.Status, res.Message)
	}

	// Binary doesn't exist
	os.Remove(binaryPath)
	res = checkPlistBinary(plistPath)
	if res.Status != Fail {
		t.Errorf("expected Fail for missing binary, got %v", res.Status)
	}

	// Malformed plist
	os.WriteFile(plistPath, []byte("not xml"), 0644)
	res = checkPlistBinary(plistPath)
	if res.Status != Fail {
		t.Errorf("expected Fail for malformed plist, got %v", res.Status)
	}
}

func TestParsePlistBinaryPath(t *testing.T) {
	// Minimal plist with just ProgramArguments
	minimal := []byte(`<?xml version="1.0"?><plist><dict><key>ProgramArguments</key><array><string>/usr/local/bin/outport</string><string>daemon</string></array></dict></plist>`)
	if got := parsePlistBinaryPath(minimal); got != "/usr/local/bin/outport" {
		t.Errorf("expected /usr/local/bin/outport, got %q", got)
	}

	// Empty input
	if got := parsePlistBinaryPath([]byte("")); got != "" {
		t.Errorf("expected empty string for empty input, got %q", got)
	}

	// No ProgramArguments key
	noProg := []byte(`<?xml version="1.0"?><plist><dict><key>Label</key><string>test</string></dict></plist>`)
	if got := parsePlistBinaryPath(noProg); got != "" {
		t.Errorf("expected empty string for missing ProgramArguments, got %q", got)
	}
}

func TestCheckCertExpiry(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "ca-cert.pem")

	// Missing file
	res := checkCertExpiry(certPath)
	if res.Status != Fail {
		t.Errorf("expected Fail for missing cert, got %v", res.Status)
	}

	// Not a valid PEM — just garbage
	os.WriteFile(certPath, []byte("not a cert"), 0644)
	res = checkCertExpiry(certPath)
	if res.Status != Fail {
		t.Errorf("expected Fail for invalid cert, got %v", res.Status)
	}
}

func TestCheckRegistryValid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "registry.json")

	// Missing file → Warn
	res := checkRegistryValid(path)
	if res.Status != Warn {
		t.Errorf("expected Warn for missing registry, got %v", res.Status)
	}

	// Valid JSON
	os.WriteFile(path, []byte(`{"projects":{}}`), 0644)
	res = checkRegistryValid(path)
	if res.Status != Pass {
		t.Errorf("expected Pass, got %v", res.Status)
	}

	// Invalid JSON → Fail
	os.WriteFile(path, []byte(`{broken`), 0644)
	res = checkRegistryValid(path)
	if res.Status != Fail {
		t.Errorf("expected Fail for corrupt registry, got %v", res.Status)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/doctor/ -v -run "TestCheck"`
Expected: FAIL — functions don't exist yet

- [ ] **Step 3: Write the check implementations**

```go
// internal/doctor/checks.go
package doctor

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/outport-app/outport/internal/certmanager"
	"github.com/outport-app/outport/internal/platform"
	"github.com/outport-app/outport/internal/portcheck"
	"github.com/outport-app/outport/internal/registry"
)

// checkFileExists checks that a file exists at the given path.
func checkFileExists(path, name, fix string) *Result {
	if _, err := os.Stat(path); err != nil {
		return &Result{
			Name:    name,
			Status:  Fail,
			Message: fmt.Sprintf("%s not found", name),
			Fix:     fix,
		}
	}
	return &Result{
		Name:    name,
		Status:  Pass,
		Message: fmt.Sprintf("%s (%s)", name, path),
	}
}

// checkResolverContent verifies the resolver file has the expected content.
func checkResolverContent(path, expected string) *Result {
	name := "DNS resolver content correct"
	data, err := os.ReadFile(path)
	if err != nil {
		return &Result{Name: name, Status: Fail, Message: "could not read resolver file", Fix: "Run: outport system start"}
	}
	if string(data) != expected {
		return &Result{Name: name, Status: Fail, Message: "resolver file has unexpected content", Fix: "Run: outport system start"}
	}
	return &Result{Name: name, Status: Pass, Message: "DNS resolver content correct"}
}

// checkPlistBinary parses the plist XML using token-based parsing to find
// the ProgramArguments array and verify the binary path exists.
// We use token-based parsing because encoding/xml struct unmarshalling
// collects <key> and <array> elements independently, breaking the
// positional correspondence needed to match keys to their values.
func checkPlistBinary(plistPath string) *Result {
	name := "LaunchAgent plist binary valid"
	data, err := os.ReadFile(plistPath)
	if err != nil {
		return &Result{Name: name, Status: Fail, Message: "could not read plist file", Fix: "Run: outport system restart"}
	}

	binaryPath := parsePlistBinaryPath(data)
	if binaryPath == "" {
		return &Result{Name: name, Status: Fail, Message: "could not find binary path in plist", Fix: "Run: outport system restart"}
	}

	if _, err := os.Stat(binaryPath); err != nil {
		return &Result{
			Name:    name,
			Status:  Fail,
			Message: fmt.Sprintf("plist references missing binary: %s", binaryPath),
			Fix:     "Run: outport system restart",
		}
	}

	return &Result{Name: name, Status: Pass, Message: "LaunchAgent plist binary valid"}
}

// parsePlistBinaryPath extracts the first string from the ProgramArguments
// array in a plist XML document using token-based parsing.
func parsePlistBinaryPath(data []byte) string {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	// Walk tokens looking for <key>ProgramArguments</key> followed by <array><string>...</string>
	for {
		tok, err := decoder.Token()
		if err != nil {
			return ""
		}
		// Look for <key> elements
		start, ok := tok.(xml.StartElement)
		if !ok || start.Name.Local != "key" {
			continue
		}
		// Read the key text
		var keyText string
		if err := decoder.DecodeElement(&keyText, &start); err != nil {
			return ""
		}
		if keyText != "ProgramArguments" {
			continue
		}
		// Next non-whitespace token should be <array>
		for {
			tok, err = decoder.Token()
			if err != nil {
				return ""
			}
			if arr, ok := tok.(xml.StartElement); ok {
				if arr.Name.Local != "array" {
					return ""
				}
				// Read first <string> inside the array
				for {
					tok, err = decoder.Token()
					if err != nil {
						return ""
					}
					if s, ok := tok.(xml.StartElement); ok {
						if s.Name.Local == "string" {
							var val string
							if err := decoder.DecodeElement(&val, &s); err != nil {
								return ""
							}
							return val
						}
					}
				}
			}
		}
	}
}

// checkAgentLoaded checks if the LaunchAgent is loaded via launchctl.
func checkAgentLoaded() *Result {
	name := "LaunchAgent loaded"
	if platform.IsAgentLoaded() {
		return &Result{Name: name, Status: Pass, Message: "LaunchAgent loaded"}
	}
	return &Result{Name: name, Status: Fail, Message: "LaunchAgent not loaded", Fix: "Run: outport system start"}
}

// checkPortUp verifies a port is accepting connections.
func checkPortUp(port int, name, fix string) *Result {
	if portcheck.IsUp(port) {
		return &Result{Name: name, Status: Pass, Message: fmt.Sprintf("%s (port %d)", name, port)}
	}
	return &Result{Name: name, Status: Fail, Message: fmt.Sprintf("%s not responding", name), Fix: fix}
}

// checkCertExpiry parses a PEM certificate and checks it hasn't expired.
func checkCertExpiry(certPath string) *Result {
	name := "CA certificate not expired"
	data, err := os.ReadFile(certPath)
	if err != nil {
		return &Result{Name: name, Status: Fail, Message: "could not read CA certificate", Fix: "Run: outport system start"}
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return &Result{Name: name, Status: Fail, Message: "CA certificate is not valid PEM", Fix: "Run: outport system uninstall && outport system start"}
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return &Result{Name: name, Status: Fail, Message: "could not parse CA certificate", Fix: "Run: outport system uninstall && outport system start"}
	}

	if time.Now().After(cert.NotAfter) {
		return &Result{
			Name:    name,
			Status:  Fail,
			Message: fmt.Sprintf("CA certificate expired on %s", cert.NotAfter.Format("2006-01-02")),
			Fix:     "Run: outport system uninstall && outport system start",
		}
	}

	return &Result{Name: name, Status: Pass, Message: fmt.Sprintf("CA certificate not expired (expires %s)", cert.NotAfter.Format("2006-01-02"))}
}

// checkCATrusted checks if the CA is trusted in the system keychain.
func checkCATrusted(certPath string) *Result {
	name := "CA trusted in system keychain"
	if platform.IsCATrusted(certPath) {
		return &Result{Name: name, Status: Pass, Message: "CA trusted in system keychain"}
	}
	return &Result{Name: name, Status: Fail, Message: "CA not trusted in system keychain", Fix: "Run: outport system start"}
}

// checkRegistryValid checks that the registry file exists and is valid JSON.
func checkRegistryValid(path string) *Result {
	name := "Registry file valid"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &Result{Name: name, Status: Warn, Message: "Registry file not found (no projects registered yet)", Fix: "Run: outport up in a project directory"}
	}

	if _, err := registry.Load(path); err != nil {
		return &Result{Name: name, Status: Fail, Message: fmt.Sprintf("Registry file is corrupt: %v", err), Fix: "Delete and re-register projects: rm ~/.local/share/outport/registry.json && outport up"}
	}

	return &Result{Name: name, Status: Pass, Message: "Registry file valid"}
}

// checkCloudflared checks if cloudflared is available in PATH.
func checkCloudflared() *Result {
	name := "cloudflared installed"
	if _, err := exec.LookPath("cloudflared"); err != nil {
		return &Result{Name: name, Status: Warn, Message: "cloudflared not installed (sharing will not work)", Fix: "Install with: brew install cloudflared"}
	}
	return &Result{Name: name, Status: Pass, Message: "cloudflared installed"}
}

// SystemChecks returns all system-level health checks.
func SystemChecks() []Check {
	caCertPath, _ := certmanager.CACertPath()
	caKeyPath, _ := certmanager.CAKeyPath()
	regPath, _ := registry.DefaultPath()
	plistPath := platform.PlistPath()

	return []Check{
		{Name: "DNS resolver file exists", Category: "System", Run: func() *Result {
			return checkFileExists(platform.ResolverPath, "DNS resolver file exists", "Run: outport system start")
		}},
		{Name: "DNS resolver content correct", Category: "System", Run: func() *Result {
			return checkResolverContent(platform.ResolverPath, platform.ResolverContent)
		}},
		{Name: "LaunchAgent plist installed", Category: "System", Run: func() *Result {
			return checkFileExists(plistPath, "LaunchAgent plist installed", "Run: outport system start")
		}},
		{Name: "LaunchAgent plist binary valid", Category: "System", Run: func() *Result {
			return checkPlistBinary(plistPath)
		}},
		{Name: "LaunchAgent loaded", Category: "System", Run: checkAgentLoaded},
		{Name: "HTTP proxy responding", Category: "System", Run: func() *Result {
			return checkPortUp(80, "HTTP proxy responding", "Run: outport system restart")
		}},
		{Name: "HTTPS proxy responding", Category: "System", Run: func() *Result {
			return checkPortUp(443, "HTTPS proxy responding", "Run: outport system restart")
		}},
		{Name: "CA certificate exists", Category: "System", Run: func() *Result {
			return checkFileExists(caCertPath, "CA certificate exists", "Run: outport system start")
		}},
		{Name: "CA private key exists", Category: "System", Run: func() *Result {
			return checkFileExists(caKeyPath, "CA private key exists", "Run: outport system start")
		}},
		{Name: "CA certificate not expired", Category: "System", Run: func() *Result {
			return checkCertExpiry(caCertPath)
		}},
		{Name: "CA trusted in system keychain", Category: "System", Run: func() *Result {
			return checkCATrusted(caCertPath)
		}},
		{Name: "Registry file valid", Category: "System", Run: func() *Result {
			return checkRegistryValid(regPath)
		}},
		{Name: "cloudflared installed", Category: "System", Run: checkCloudflared},
	}
}
```

Note: DNS resolution check (check #6 from the spec) is deferred to Task 4 as it requires `miekg/dns`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/doctor/ -v`
Expected: PASS

- [ ] **Step 5: Run lint**

Run: `golangci-lint run ./internal/doctor/`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/doctor/checks.go internal/doctor/checks_test.go
git commit -m "feat(doctor): add system check implementations"
```

---

### Task 4: DNS resolution check

**Files:**
- Create: `internal/doctor/dns.go`
- Modify: `internal/doctor/checks.go` (add DNS check to `SystemChecks()`)

- [ ] **Step 1: Write the DNS check function**

```go
// internal/doctor/dns.go
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
```

- [ ] **Step 2: Add DNS check to SystemChecks in checks.go**

Insert the DNS check between "LaunchAgent loaded" and "HTTP proxy responding" in the `SystemChecks()` slice:

```go
{Name: "DNS resolving *.test", Category: "System", Run: func() *Result {
	return checkDNSResolving("127.0.0.1:15353")
}},
```

- [ ] **Step 3: Run tests and lint**

Run: `go test ./internal/doctor/ -v && golangci-lint run ./internal/doctor/`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/doctor/dns.go internal/doctor/checks.go
git commit -m "feat(doctor): add DNS resolution check"
```

---

### Task 5: Project checks

**Files:**
- Create: `internal/doctor/project.go`
- Create: `internal/doctor/project_test.go`

- [ ] **Step 1: Write failing tests for project checks**

```go
// internal/doctor/project_test.go
package doctor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckConfigValid(t *testing.T) {
	dir := t.TempDir()

	// Valid config
	cfg := `name: myapp
services:
  web:
    env_var: PORT
`
	os.WriteFile(filepath.Join(dir, ".outport.yml"), []byte(cfg), 0644)
	res := checkConfigValid(dir)
	if res.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", res.Status, res.Message)
	}

	// Invalid config (missing name)
	os.WriteFile(filepath.Join(dir, ".outport.yml"), []byte("services:\n  web:\n    env_var: PORT\n"), 0644)
	res = checkConfigValid(dir)
	if res.Status != Fail {
		t.Errorf("expected Fail, got %v", res.Status)
	}
}

func TestCheckProjectRegistered(t *testing.T) {
	dir := t.TempDir()
	regPath := filepath.Join(dir, "registry.json")

	// Empty registry
	os.WriteFile(regPath, []byte(`{"projects":{}}`), 0644)
	res := checkProjectRegistered(regPath, dir)
	if res.Status != Fail {
		t.Errorf("expected Fail for unregistered project, got %v", res.Status)
	}

	// Registered
	os.WriteFile(regPath, []byte(`{"projects":{"myapp/main":{"project_dir":"`+dir+`","ports":{"web":3000}}}}`), 0644)
	res = checkProjectRegistered(regPath, dir)
	if res.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", res.Status, res.Message)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/doctor/ -v -run "TestCheckConfig|TestCheckProject"`
Expected: FAIL

- [ ] **Step 3: Write the project check implementations**

```go
// internal/doctor/project.go
package doctor

import (
	"fmt"
	"sort"

	"github.com/outport-app/outport/internal/config"
	"github.com/outport-app/outport/internal/portcheck"
	"github.com/outport-app/outport/internal/registry"
)

// checkConfigValid attempts to load and validate the .outport.yml in dir.
func checkConfigValid(dir string) *Result {
	name := ".outport.yml valid"
	_, err := config.Load(dir)
	if err != nil {
		return &Result{Name: name, Status: Fail, Message: fmt.Sprintf(".outport.yml: %v", err)}
	}
	return &Result{Name: name, Status: Pass, Message: ".outport.yml valid"}
}

// checkProjectRegistered checks if the current directory is registered in the registry.
func checkProjectRegistered(regPath, dir string) *Result {
	name := "Project registered"
	reg, err := registry.Load(regPath)
	if err != nil {
		return &Result{Name: name, Status: Fail, Message: fmt.Sprintf("could not load registry: %v", err)}
	}
	key, _, found := reg.FindByDir(dir)
	if !found {
		return &Result{Name: name, Status: Fail, Message: "project not registered", Fix: "Run: outport up"}
	}
	return &Result{Name: name, Status: Pass, Message: fmt.Sprintf("Project registered (%s)", key)}
}

// checkPortAvailable checks if an allocated port is in use.
// Returns Warn (not Fail) because the service itself may be running.
func checkPortAvailable(port int, serviceName string) *Result {
	name := fmt.Sprintf("Port %d (%s)", port, serviceName)
	if portcheck.IsUp(port) {
		return &Result{Name: name, Status: Warn, Message: fmt.Sprintf("Port %d (%s) is in use", port, serviceName)}
	}
	return &Result{Name: name, Status: Pass, Message: fmt.Sprintf("Port %d (%s) available", port, serviceName)}
}

// ProjectChecks returns project-level health checks for the given directory.
// cfg may be nil if config loading failed (in which case only the config check is returned).
// regPath is the path to registry.json.
func ProjectChecks(dir string, cfg *config.Config, configErr error, regPath string) []Check {
	category := "Project"
	if cfg != nil {
		category = fmt.Sprintf("Project (%s)", cfg.Name)
	}

	var checks []Check

	// Config validity check
	if configErr != nil {
		checks = append(checks, Check{
			Name:     ".outport.yml valid",
			Category: category,
			Run: func() *Result {
				return &Result{Name: ".outport.yml valid", Status: Fail, Message: fmt.Sprintf(".outport.yml: %v", configErr)}
			},
		})
		return checks // Skip remaining project checks
	}

	checks = append(checks, Check{
		Name:     ".outport.yml valid",
		Category: category,
		Run: func() *Result {
			return &Result{Name: ".outport.yml valid", Status: Pass, Message: ".outport.yml valid"}
		},
	})

	// Registration check
	checks = append(checks, Check{
		Name:     "Project registered",
		Category: category,
		Run: func() *Result {
			return checkProjectRegistered(regPath, dir)
		},
	})

	// Port checks — load registry to get allocated ports
	reg, err := registry.Load(regPath)
	if err == nil {
		if _, alloc, found := reg.FindByDir(dir); found {
			serviceNames := make([]string, 0, len(alloc.Ports))
			for name := range alloc.Ports {
				serviceNames = append(serviceNames, name)
			}
			sort.Strings(serviceNames)
			for _, svcName := range serviceNames {
				port := alloc.Ports[svcName]
				svc := svcName // capture for closure
				p := port
				checks = append(checks, Check{
					Name:     fmt.Sprintf("Port %d (%s)", p, svc),
					Category: category,
					Run: func() *Result {
						return checkPortAvailable(p, svc)
					},
				})
			}
		}
	}

	return checks
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/doctor/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/doctor/project.go internal/doctor/project_test.go
git commit -m "feat(doctor): add project-level checks"
```

---

### Task 6: `cmd/doctor.go` — Command wiring and output

**Files:**
- Modify: `cmd/cmdutil.go` (add `SilentError` sentinel)
- Modify: `main.go` (suppress output for `SilentError`)
- Create: `cmd/doctor.go`

- [ ] **Step 1: Add SilentError sentinel to cmdutil.go**

Add to `cmd/cmdutil.go`:

```go
// SilentError is returned when a command wants to set exit code 1
// without printing an error message. main.go checks for this.
var SilentError = errors.New("")
```

Update `main.go` to suppress output for SilentError:

```go
func main() {
	if err := cmd.Execute(); err != nil {
		if err != cmd.SilentError {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}
```

- [ ] **Step 2: Write the command**

```go
// cmd/doctor.go
package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"charm.land/lipgloss/v2"
	"github.com/outport-app/outport/internal/config"
	"github.com/outport-app/outport/internal/doctor"
	"github.com/outport-app/outport/internal/registry"
	"github.com/outport-app/outport/internal/ui"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:     "doctor",
	Short:   "Check the health of the outport system",
	Long:    "Runs diagnostic checks on DNS, daemon, certificates, registry, and project configuration. Reports pass/warn/fail for each check with actionable fix suggestions.",
	GroupID: "project",
	Args:    NoArgs,
	RunE:    runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

func runDoctor(cmd *cobra.Command, args []string) error {
	r := &doctor.Runner{}

	// System checks (always)
	for _, c := range doctor.SystemChecks() {
		r.Add(c)
	}

	// Project checks (when .outport.yml found)
	cwd, err := os.Getwd()
	if err == nil {
		if dir, findErr := config.FindDir(cwd); findErr == nil {
			regPath, _ := registry.DefaultPath()
			cfg, configErr := config.Load(dir)
			for _, c := range doctor.ProjectChecks(dir, cfg, configErr, regPath) {
				r.Add(c)
			}
		}
	}

	results := r.Run()

	if jsonFlag {
		return printDoctorJSON(cmd, results)
	}

	printDoctorStyled(cmd.OutOrStdout(), results)

	if doctor.HasFailures(results) {
		return SilentError
	}
	return nil
}

// JSON output

type resultJSON struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Status   string `json:"status"`
	Message  string `json:"message"`
	Fix      string `json:"fix,omitempty"`
}

type doctorJSON struct {
	Results []resultJSON `json:"results"`
	Passed  bool         `json:"passed"`
}

func printDoctorJSON(cmd *cobra.Command, results []doctor.Result) error {
	out := doctorJSON{
		Passed: !doctor.HasFailures(results),
	}
	for _, r := range results {
		out.Results = append(out.Results, resultJSON{
			Name:     r.Name,
			Category: r.Category,
			Status:   r.Status.String(),
			Message:  r.Message,
			Fix:      r.Fix,
		})
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

// Styled output

func printDoctorStyled(w io.Writer, results []doctor.Result) {
	currentCategory := ""
	for _, r := range results {
		if r.Category != currentCategory {
			if currentCategory != "" {
				lipgloss.Fprintln(w) // blank line between categories
			}
			lipgloss.Fprintln(w, ui.ProjectStyle.Render(r.Category))
			currentCategory = r.Category
		}

		var icon string
		switch r.Status {
		case doctor.Pass:
			icon = lipgloss.NewStyle().Foreground(ui.Green).Render("✓")
		case doctor.Warn:
			icon = lipgloss.NewStyle().Foreground(ui.Yellow).Render("!")
		case doctor.Fail:
			icon = lipgloss.NewStyle().Foreground(ui.Red).Render("✗")
		}

		lipgloss.Fprintln(w, fmt.Sprintf("  %s %s", icon, r.Message))

		if r.Fix != "" {
			lipgloss.Fprintln(w, fmt.Sprintf("    %s %s", ui.Arrow, ui.DimStyle.Render(r.Fix)))
		}
	}

	lipgloss.Fprintln(w)
	if doctor.HasFailures(results) {
		lipgloss.Fprintln(w, lipgloss.NewStyle().Foreground(ui.Red).Render("Some checks failed. See suggestions above."))
	} else {
		lipgloss.Fprintln(w, ui.SuccessStyle.Render("All checks passed."))
	}
}
```

- [ ] **Step 2: Build and verify it compiles**

Run: `just build`
Expected: Compiles without errors

- [ ] **Step 3: Run lint**

Run: `golangci-lint run ./cmd/ ./internal/doctor/`
Expected: PASS

- [ ] **Step 4: Manual smoke test**

Run: `just run doctor`
Expected: Output showing system checks with ✓/✗/! indicators

Run: `just run doctor --json`
Expected: JSON output with results array and passed field

- [ ] **Step 5: Commit**

```bash
git add cmd/doctor.go
git commit -m "feat: add outport doctor command

Diagnostic command that checks the health of all Outport infrastructure:
DNS resolver, daemon, CA certificates, registry, and project config.
Supports --json for machine-readable output.

Closes #35"
```

---

### Task 7: Full test suite + lint verification

**Files:** None (verification only)

- [ ] **Step 1: Run full test suite**

Run: `just test`
Expected: All tests pass

- [ ] **Step 2: Run lint**

Run: `just lint`
Expected: No lint errors

- [ ] **Step 3: Verify --json output**

Run: `just run doctor --json | python3 -m json.tool`
Expected: Valid JSON with proper structure
