package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Allocation struct {
	ProjectDir            string            `json:"project_dir"`
	Ports                 map[string]int    `json:"ports"`
	Hostnames             map[string]string `json:"hostnames,omitempty"`
	Protocols             map[string]string `json:"protocols,omitempty"`
	EnvVars               map[string]string `json:"env_vars,omitempty"`
	ApprovedExternalFiles []string          `json:"approved_external_files,omitempty"`
}

func registryKey(project, instance string) string {
	return project + "/" + instance
}

// ParseKey splits a registry key ("project/instance") into its components.
func ParseKey(key string) (project, instance string) {
	parts := strings.SplitN(key, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return key, "main"
}

type Registry struct {
	Projects map[string]Allocation `json:"projects"`
	path     string
}

func Load(path string) (*Registry, error) {
	reg := &Registry{
		Projects: make(map[string]Allocation),
		path:     path,
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return reg, nil
		}
		return nil, fmt.Errorf("reading registry: %w", err)
	}

	if err := json.Unmarshal(data, reg); err != nil {
		return nil, fmt.Errorf("parsing registry: %w", err)
	}
	if reg.Projects == nil {
		reg.Projects = make(map[string]Allocation)
	}
	for key, alloc := range reg.Projects {
		if alloc.Hostnames == nil {
			alloc.Hostnames = make(map[string]string)
		}
		if alloc.Protocols == nil {
			alloc.Protocols = make(map[string]string)
		}
		reg.Projects[key] = alloc
	}
	reg.path = path

	return reg, nil
}

func (r *Registry) Save() error {
	dir := filepath.Dir(r.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating registry dir: %w", err)
	}

	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling registry: %w", err)
	}

	tmp := r.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("writing registry: %w", err)
	}
	if err := os.Rename(tmp, r.path); err != nil {
		return fmt.Errorf("renaming registry: %w", err)
	}

	return nil
}

func (r *Registry) Set(project, instance string, alloc Allocation) {
	r.Projects[registryKey(project, instance)] = alloc
}

func (r *Registry) Get(project, instance string) (Allocation, bool) {
	alloc, ok := r.Projects[registryKey(project, instance)]
	return alloc, ok
}

func (r *Registry) Remove(project, instance string) {
	delete(r.Projects, registryKey(project, instance))
}

// FindByDir searches for an allocation whose ProjectDir matches the given directory.
func (r *Registry) FindByDir(dir string) (string, Allocation, bool) {
	for key, alloc := range r.Projects {
		if alloc.ProjectDir == dir {
			return key, alloc, true
		}
	}
	return "", Allocation{}, false
}

// FindByProject returns all registry keys that belong to the given project name.
func (r *Registry) FindByProject(project string) map[string]Allocation {
	prefix := project + "/"
	result := make(map[string]Allocation)
	for key, alloc := range r.Projects {
		if strings.HasPrefix(key, prefix) {
			result[key] = alloc
		}
	}
	return result
}

func (r *Registry) UsedPorts() map[int]bool {
	used := make(map[int]bool)
	for _, alloc := range r.Projects {
		for _, port := range alloc.Ports {
			used[port] = true
		}
	}
	return used
}
