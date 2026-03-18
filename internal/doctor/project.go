package doctor

import (
	"fmt"
	"sort"

	"github.com/outport-app/outport/internal/config"
	"github.com/outport-app/outport/internal/portcheck"
	"github.com/outport-app/outport/internal/registry"
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
		serviceNames := make([]string, 0, len(alloc.Ports))
		for name := range alloc.Ports {
			serviceNames = append(serviceNames, name)
		}
		sort.Strings(serviceNames)
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
	}

	return checks
}
