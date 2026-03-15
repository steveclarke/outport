package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Allocation struct {
	ProjectDir string            `json:"project_dir"`
	Ports      map[string]int    `json:"ports"`
	Hostnames  map[string]string `json:"hostnames,omitempty"`
	Protocols  map[string]string `json:"protocols,omitempty"`
}

func registryKey(project, instance string) string {
	return project + "/" + instance
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

func (r *Registry) UsedPorts() map[int]bool {
	used := make(map[int]bool)
	for _, alloc := range r.Projects {
		for _, port := range alloc.Ports {
			used[port] = true
		}
	}
	return used
}
