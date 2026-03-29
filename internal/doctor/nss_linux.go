//go:build linux

package doctor

import (
	"fmt"
	"strings"

	"github.com/steveclarke/outport/internal/platform"
)

// linuxBrowserTrustChecks returns checks that verify the outport CA is
// trusted in browser NSS databases (Chrome, Firefox) on Linux.
func linuxBrowserTrustChecks() []Check {
	return []Check{
		{
			Name:     "certutil installed",
			Category: "TLS",
			Run:      checkCertutilInstalled,
		},
		{
			Name:     "Browser CA trust",
			Category: "TLS",
			Run:      checkBrowserCATrust,
		},
	}
}

func checkCertutilInstalled() *Result {
	if platform.HasCertutil() {
		return &Result{
			Name:    "certutil installed",
			Status:  Pass,
			Message: "certutil is available",
		}
	}
	return &Result{
		Name:    "certutil installed",
		Status:  Warn,
		Message: "certutil not found — browsers may not trust .test certificates",
		Fix:     "Install: " + platform.CertutilInstallHint(),
	}
}

func checkBrowserCATrust() *Result {
	name := "Browser CA trust"

	if !platform.HasCertutil() {
		return &Result{
			Name:    name,
			Status:  Warn,
			Message: "cannot check browser trust without certutil",
		}
	}

	dbs := platform.FindNSSDatabases()
	if len(dbs) == 0 {
		return &Result{
			Name:    name,
			Status:  Pass,
			Message: "no browser NSS databases found (browsers not installed or not yet launched)",
		}
	}

	var untrusted []string
	for _, db := range dbs {
		if !platform.IsNSSTrusted(db.Path) {
			untrusted = append(untrusted, db.Description)
		}
	}

	if len(untrusted) == 0 {
		return &Result{
			Name:    name,
			Status:  Pass,
			Message: fmt.Sprintf("CA trusted in %d browser database(s)", len(dbs)),
		}
	}

	return &Result{
		Name:    name,
		Status:  Warn,
		Message: fmt.Sprintf("CA not trusted in: %s", joinNames(untrusted)),
		Fix:     "Run: outport system restart",
	}
}

func joinNames(names []string) string {
	switch len(names) {
	case 1:
		return names[0]
	case 2:
		return names[0] + " and " + names[1]
	default:
		return strings.Join(names[:len(names)-1], ", ") + ", and " + names[len(names)-1]
	}
}
