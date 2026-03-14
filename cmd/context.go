package cmd

import (
	"fmt"
	"os"

	"github.com/outport-app/outport/internal/config"
	"github.com/outport-app/outport/internal/registry"
	"github.com/outport-app/outport/internal/worktree"
)

type projectContext struct {
	Dir string
	Cfg *config.Config
	WT  *worktree.Info
	Reg *registry.Registry
}

func loadProjectContext() (*projectContext, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("Getting working directory: %w.", err)
	}
	cfg, err := config.Load(dir)
	if err != nil {
		return nil, err
	}
	wt, err := worktree.Detect(dir)
	if err != nil {
		return nil, fmt.Errorf("Detecting worktree: %w.", err)
	}
	regPath, err := registry.DefaultPath()
	if err != nil {
		return nil, err
	}
	reg, err := registry.Load(regPath)
	if err != nil {
		return nil, err
	}
	return &projectContext{Dir: dir, Cfg: cfg, WT: wt, Reg: reg}, nil
}

func loadRegistry() (*registry.Registry, error) {
	regPath, err := registry.DefaultPath()
	if err != nil {
		return nil, err
	}
	return registry.Load(regPath)
}

func hasGroups(cfg *config.Config, serviceNames []string) bool {
	for _, name := range serviceNames {
		if svc, ok := cfg.Services[name]; ok && svc.Group != "" {
			return true
		}
	}
	return false
}
