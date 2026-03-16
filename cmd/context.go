package cmd

import (
	"fmt"
	"os"

	"github.com/outport-app/outport/internal/config"
	"github.com/outport-app/outport/internal/instance"
	"github.com/outport-app/outport/internal/registry"
)

type projectContext struct {
	Dir      string
	Cfg      *config.Config
	Instance string
	IsNew    bool
	Reg      *registry.Registry
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
	}, nil
}

func loadRegistry() (*registry.Registry, error) {
	regPath, err := registry.DefaultPath()
	if err != nil {
		return nil, err
	}
	return registry.Load(regPath)
}
