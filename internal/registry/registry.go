// Package registry provides the persistent JSON store that tracks all registered
// Outport projects and their port/hostname allocations. The registry file lives at
// ~/.local/share/outport/registry.json and is the single source of truth for which
// projects exist, what ports and hostnames they own, and where their project
// directories are on disk.
//
// Registry keys use the format "project/instance" (e.g., "myapp/main" or
// "myapp/bxcf"). Each key maps to an Allocation containing the project's
// directory path, assigned ports, hostnames, environment variable mappings,
// and any approved external env file paths.
//
// The registry uses atomic writes (write to temp file, then rename) to prevent
// corruption from crashes or concurrent access. The daemon watches the registry
// file for changes and rebuilds its routing table when it detects modifications.
package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Allocation represents a single project instance's resource assignments in the
// registry. Each registered project directory gets one Allocation, storing
// everything the daemon and CLI need to manage that project's local dev services.
type Allocation struct {
	// ProjectDir is the absolute filesystem path to the project's root directory.
	// Used by FindByDir to look up which project owns a given directory, and by
	// RemoveStale to detect projects whose directories no longer exist.
	ProjectDir string `json:"project_dir"`

	// Ports maps service names (as defined in outport.yml) to their assigned port
	// numbers. For example, {"web": 13542, "api": 28901}. Ports are deterministically
	// assigned by the allocator using FNV-32a hashing and persist across runs.
	Ports map[string]int `json:"ports"`

	// Hostnames maps service names to their .test domain hostnames. Only services
	// that declare a hostname in outport.yml get entries here. For example,
	// {"web": "myapp.test"}. Non-main instances get suffixed hostnames like
	// "myapp-bxcf.test" to ensure global uniqueness.
	Hostnames map[string]string `json:"hostnames,omitempty"`

	// Aliases maps service names to their named alias hostnames. Each service can
	// have zero or more aliases, keyed by alias name (e.g., {"web": {"app": "app.myapp.test"}}).
	// Aliases register additional proxy routes to the same port as the primary hostname.
	Aliases map[string]map[string]string `json:"aliases,omitempty"`

	// Subdomains maps service names to a boolean indicating wildcard subdomain
	// routing is enabled. When true for a service, all subdomains of its primary
	// hostname route to the same port (e.g., *.myapp.test → port).
	Subdomains map[string]bool `json:"subdomains,omitempty"`

	// EnvVars maps environment variable names to their computed values after
	// template expansion. These are the key=value pairs written into .env files
	// by the dotenv package. For example, {"PORT": "13542", "DATABASE_URL": "..."}.
	EnvVars map[string]string `json:"env_vars,omitempty"`

	// ApprovedExternalFiles lists env file paths that fall outside the project
	// directory but have been explicitly approved by the developer. Outport
	// requires approval before writing to files outside the project boundary
	// as a safety measure. These paths are remembered so subsequent runs of
	// "outport up" do not re-prompt.
	ApprovedExternalFiles []string `json:"approved_external_files,omitempty"`
}

func registryKey(project, instance string) string {
	return project + "/" + instance
}

// Key constructs a registry key in "project/instance" format from separate
// project and instance name strings. This is the exported version of
// registryKey, used by other packages that need to build or compare keys.
func Key(project, instance string) string {
	return registryKey(project, instance)
}

// ParseKey splits a registry key ("project/instance") into its project and
// instance components. If the key does not contain a slash (which should not
// happen in normal operation), it returns the entire key as the project name
// and defaults the instance to "main".
func ParseKey(key string) (project, instance string) {
	parts := strings.SplitN(key, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return key, "main"
}

// Registry is the in-memory representation of the registry.json file. It holds
// all project allocations keyed by "project/instance" strings, along with the
// file path used for loading and saving. All mutation methods (Set, Remove,
// RemoveStale) operate on the in-memory map only — call Save to persist changes
// to disk.
type Registry struct {
	// Projects maps registry keys ("project/instance") to their Allocation data.
	// This is the top-level JSON object in the registry file.
	Projects map[string]Allocation `json:"projects"`

	// path is the filesystem location of the registry JSON file, set during Load.
	// Used by Save to write back to the same location. Not exported because
	// callers should not change the path after loading.
	path string
}

// Load reads and parses the registry JSON file at the given path. If the file
// does not exist, Load returns an empty registry (not an error), allowing
// first-run scenarios to work without setup. All Hostnames maps are initialized
// to non-nil values so callers can safely read from them without nil checks.
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
		reg.Projects[key] = alloc
	}
	reg.path = path

	return reg, nil
}

// Save writes the registry to disk as pretty-printed JSON. It uses atomic
// writes (write to a .tmp file, then rename) to prevent corruption if the
// process is interrupted mid-write. The parent directory is created if it
// does not exist. The daemon watches this file for changes and rebuilds its
// DNS and proxy routing tables whenever the file is modified.
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

// Set stores an allocation for the given project and instance, replacing any
// existing entry with the same key. This only modifies the in-memory map;
// call Save to persist the change to disk.
func (r *Registry) Set(project, instance string, alloc Allocation) {
	r.Projects[registryKey(project, instance)] = alloc
}

// Get retrieves the allocation for a specific project and instance. The boolean
// return value indicates whether the entry was found. Returns a zero-value
// Allocation if not found.
func (r *Registry) Get(project, instance string) (Allocation, bool) {
	alloc, ok := r.Projects[registryKey(project, instance)]
	return alloc, ok
}

// Remove deletes the allocation for a specific project and instance from the
// in-memory map. This is called by "outport down" to unregister a project.
// Call Save afterward to persist the removal to disk.
func (r *Registry) Remove(project, instance string) {
	delete(r.Projects, registryKey(project, instance))
}

// FindByDir searches for an allocation whose ProjectDir matches the given
// absolute directory path. Returns the registry key, the allocation, and true
// if found. This is the primary way CLI commands identify which project they
// are operating on — they resolve the current working directory and look it up
// in the registry. Returns zero values and false if no match is found.
//
// Matching is filesystem-aware. A plain string match is tried first, but on a
// miss the lookup falls back to os.SameFile so that a different spelling of the
// same directory resolves to the existing instance. This matters on
// case-insensitive filesystems (macOS, Windows): os.Getwd can report the
// project path with different casing than the registry stored (e.g.
// ".../Connon" vs ".../connon"), and a string compare would miss and register
// a phantom instance. os.SameFile compares device+inode, so it is correct on
// case-sensitive filesystems too (distinct dirs stay distinct). When several
// entries point at the same directory (a registry already polluted by this
// bug), the "main" instance wins so the lookup is deterministic and self-heals.
func (r *Registry) FindByDir(dir string) (string, Allocation, bool) {
	// Fast path: exact string match.
	for key, alloc := range r.Projects {
		if alloc.ProjectDir == dir {
			return key, alloc, true
		}
	}

	// Fallback: match by filesystem identity.
	dirInfo, err := os.Stat(dir)
	if err != nil {
		return "", Allocation{}, false
	}
	var fbKey string
	var fbAlloc Allocation
	found := false
	for key, alloc := range r.Projects {
		info, err := os.Stat(alloc.ProjectDir)
		if err != nil {
			continue
		}
		if !os.SameFile(dirInfo, info) {
			continue
		}
		if instanceOf(key) == "main" {
			return key, alloc, true
		}
		if !found {
			fbKey, fbAlloc, found = key, alloc, true
		}
	}
	return fbKey, fbAlloc, found
}

// instanceOf returns the instance portion of a "project/instance" registry key.
func instanceOf(key string) string {
	parts := strings.SplitN(key, "/", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return ""
}

// PruneDuplicateDirs removes redundant registry entries that point at the same
// filesystem directory as another entry — the phantom instances older versions
// could create when os.Getwd reported a project path with different casing than
// the registry had stored (see FindByDir). FindByDir routes around these at
// lookup time; this removes them from the store so the pollution actually heals.
//
// Entries are grouped by filesystem identity via os.SameFile. For each group of
// two or more entries resolving to one directory, the canonical entry is kept
// (the "main" instance wins; otherwise the lexicographically smallest key) and
// the rest are deleted. A single entry per directory is never touched, so
// legitimate worktrees and clones (distinct directories, distinct inodes) are
// unaffected. Entries whose ProjectDir no longer exists on disk are also left
// alone — pruning those is RemoveStale's job. Returns the removed keys (sorted)
// for caller feedback. Only mutates the in-memory map; call Save to persist.
func (r *Registry) PruneDuplicateDirs() []string {
	type entry struct {
		key  string
		info os.FileInfo
	}
	var entries []entry
	for key, alloc := range r.Projects {
		info, err := os.Stat(alloc.ProjectDir)
		if err != nil {
			continue // missing dir — leave for RemoveStale
		}
		entries = append(entries, entry{key, info})
	}

	var removed []string
	used := make([]bool, len(entries))
	for i := range entries {
		if used[i] {
			continue
		}
		group := []string{entries[i].key}
		used[i] = true
		for j := i + 1; j < len(entries); j++ {
			if used[j] {
				continue
			}
			if os.SameFile(entries[i].info, entries[j].info) {
				group = append(group, entries[j].key)
				used[j] = true
			}
		}
		if len(group) < 2 {
			continue
		}
		keep := canonicalKey(group)
		for _, key := range group {
			if key != keep {
				delete(r.Projects, key)
				removed = append(removed, key)
			}
		}
	}
	sort.Strings(removed)
	return removed
}

// canonicalKey picks the entry to keep from a set of registry keys that all
// point at the same directory: the "main" instance if present, otherwise the
// lexicographically smallest key (a stable, deterministic choice).
func canonicalKey(keys []string) string {
	best := ""
	for _, key := range keys {
		if instanceOf(key) == "main" {
			return key
		}
		if best == "" || key < best {
			best = key
		}
	}
	return best
}

// FindByProject returns all allocations whose registry keys start with the given
// project name. This finds every instance of a project (main, worktrees, clones).
// Used by the instance package to check whether a project already has a main
// instance, and by commands like "outport list" to show all instances of a project.
// Returns an empty map if no instances are found.
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

// All returns a shallow copy of the projects map. The returned map can be safely
// iterated and modified without affecting the registry's internal state. Used by
// the daemon to build routing tables and by "outport list --all" to display every
// registered project.
func (r *Registry) All() map[string]Allocation {
	result := make(map[string]Allocation, len(r.Projects))
	for k, v := range r.Projects {
		result[k] = v
	}
	return result
}

// FindHostname checks whether a .test hostname is already allocated to any
// project/instance other than excludeKey. The excludeKey parameter allows the
// caller to skip the current project's own entry (since a project re-registering
// the same hostname is not a conflict). Returns the conflicting registry key and
// true if a conflict is found, or empty string and false if the hostname is
// available. This is used during "outport up" to enforce global hostname
// uniqueness across all registered projects.
func (r *Registry) FindHostname(hostname, excludeKey string) (string, bool) {
	for key, alloc := range r.Projects {
		if key == excludeKey {
			continue
		}
		for _, h := range alloc.Hostnames {
			if h == hostname {
				return key, true
			}
		}
		for _, svcAliases := range alloc.Aliases {
			for _, h := range svcAliases {
				if h == hostname {
					return key, true
				}
			}
		}
	}
	return "", false
}

// RemoveStale removes all registry entries for which the provided predicate
// function returns true when called with the entry's ProjectDir. This is used
// by "outport system prune" to clean up entries whose project directories no
// longer exist on disk (e.g., deleted clones or removed worktrees). Returns the
// list of registry keys that were removed, which the caller can use for logging
// or user feedback. Call Save afterward to persist the removals.
func (r *Registry) RemoveStale(isStale func(projectDir string) bool) []string {
	var removed []string
	for key, alloc := range r.Projects {
		if isStale(alloc.ProjectDir) {
			removed = append(removed, key)
			delete(r.Projects, key)
		}
	}
	return removed
}

// UsedPorts returns a set of all port numbers currently assigned across every
// project and instance in the registry. The allocator uses this to detect
// collisions when assigning ports to new services — if a hash-derived port is
// already in use by another project, linear probing finds the next available one.
func (r *Registry) UsedPorts() map[int]bool {
	used := make(map[int]bool)
	for _, alloc := range r.Projects {
		for _, port := range alloc.Ports {
			used[port] = true
		}
	}
	return used
}
