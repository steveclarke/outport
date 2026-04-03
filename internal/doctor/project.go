package doctor

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/steveclarke/outport/internal/config"
	"github.com/steveclarke/outport/internal/portcheck"
	"github.com/steveclarke/outport/internal/portinfo"
	"github.com/steveclarke/outport/internal/registry"
)

// checkPortStatus reports whether an allocated port is in use (service running)
// or not (service stopped). Both are Pass — this is informational, not a problem.
func checkPortStatus(port int, serviceName string) *Result {
	name := fmt.Sprintf("Port %d (%s)", port, serviceName)
	if portcheck.IsUp(port) {
		return &Result{Name: name, Status: Pass, Message: fmt.Sprintf("Port %d (%s) in use", port, serviceName)}
	}
	return &Result{Name: name, Status: Pass, Message: fmt.Sprintf("Port %d (%s) not running", port, serviceName)}
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
			Name:     "outport.yml valid",
			Category: category,
			Run: func() *Result {
				return &Result{Name: "outport.yml valid", Status: Fail, Message: fmt.Sprintf("outport.yml: %v", configErr)}
			},
		})
		return checks // Skip remaining project checks
	}

	checks = append(checks, Check{
		Name:     "outport.yml valid",
		Category: category,
		Run: func() *Result {
			return &Result{Name: "outport.yml valid", Status: Pass, Message: "outport.yml valid"}
		},
	})

	// Load registry once for both registration check and port checks
	reg, err := registry.Load(regPath)
	if err != nil {
		return checks
	}

	key, alloc, found := reg.FindByDir(dir)

	// Registration check
	checks = append(checks, Check{
		Name:     "Project registered",
		Category: category,
		Run: func() *Result {
			if !found {
				return &Result{Name: "Project registered", Status: Fail, Message: "project not registered", Fix: "Run: outport up"}
			}
			return &Result{Name: "Project registered", Status: Pass, Message: fmt.Sprintf("Project registered (%s)", key)}
		},
	})

	// Port checks
	if found {
		serviceNames := slices.Sorted(maps.Keys(alloc.Ports))
		for _, svcName := range serviceNames {
			port := alloc.Ports[svcName]
			checks = append(checks, Check{
				Name:     fmt.Sprintf("Port %d (%s)", port, svcName),
				Category: category,
				Run: func() *Result {
					return checkPortStatus(port, svcName)
				},
			})
		}

		// Orphan check — scan managed ports for orphaned/zombie processes
		allPorts := make([]int, 0, len(alloc.Ports))
		for _, port := range alloc.Ports {
			allPorts = append(allPorts, port)
		}
		checks = append(checks, Check{
			Name:     "Orphaned processes",
			Category: "Ports",
			Run: func() *Result {
				processes, err := portinfo.ScanPorts(allPorts, portinfo.SystemScanner{})
				if err != nil {
					return &Result{
						Name:    "Orphaned processes",
						Status:  Warn,
						Message: fmt.Sprintf("could not scan ports: %v", err),
					}
				}
				var orphanPorts []string
				for _, p := range processes {
					if p.IsOrphan || p.IsZombie {
						orphanPorts = append(orphanPorts, fmt.Sprintf("%d (%s)", p.Port, p.Name))
					}
				}
				if len(orphanPorts) > 0 {
					return &Result{
						Name:    "Orphaned processes",
						Status:  Warn,
						Message: fmt.Sprintf("orphaned processes on: %s", strings.Join(orphanPorts, ", ")),
						Fix:     "Run: outport ports kill --orphans",
					}
				}
				return &Result{
					Name:    "Orphaned processes",
					Status:  Pass,
					Message: "no orphaned processes on managed ports",
				}
			},
		})
	}

	return checks
}
