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
