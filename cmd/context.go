package cmd

import (
	"fmt"
	"os"

	"github.com/steveclarke/outport/internal/config"
	"github.com/steveclarke/outport/internal/instance"
	"github.com/steveclarke/outport/internal/registry"
)

type projectContext struct {
	Dir      string
	Cfg      *config.Config
	Instance string
	IsNew    bool
	Reg      *registry.Registry
	// Pruned lists registry keys removed as duplicate-directory phantoms while
	// loading (see registry.PruneDuplicateDirs). The removal is only persisted
	// when the command saves the registry; commands may surface this to the user.
	Pruned []string
}

func loadProjectContext() (*projectContext, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}

	dir, err := config.FindDir(cwd)
	if err != nil {
		return nil, err
	}

	cfg, err := config.Load(dir)
	if err != nil {
		return nil, err
	}

	reg, err := loadRegistry()
	if err != nil {
		return nil, err
	}

	// Heal any duplicate-directory phantoms before resolving the instance, so
	// resolution sees a deduped registry (otherwise a command could re-Set a
	// phantom it just pruned). Persisted when the command saves the registry.
	pruned := reg.PruneDuplicateDirs()

	inst, isNew, err := instance.Resolve(reg, cfg.Name, dir)
	if err != nil {
		return nil, err
	}

	return &projectContext{
		Dir:      dir,
		Cfg:      cfg,
		Instance: inst,
		IsNew:    isNew,
		Reg:      reg,
		Pruned:   pruned,
	}, nil
}

func loadRegistry() (*registry.Registry, error) {
	regPath, err := registry.DefaultPath()
	if err != nil {
		return nil, err
	}
	return registry.Load(regPath)
}
