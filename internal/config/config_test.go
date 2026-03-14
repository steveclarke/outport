package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".outport.yml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLoad_SimpleConfig(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    preferred_port: 3000
    env_var: PORT
  postgres:
    preferred_port: 5432
    env_var: DATABASE_PORT
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Name != "myapp" {
		t.Errorf("name = %q, want %q", cfg.Name, "myapp")
	}
	if len(cfg.Services) != 2 {
		t.Fatalf("services count = %d, want 2", len(cfg.Services))
	}
	web := cfg.Services["web"]
	if web.PreferredPort != 3000 {
		t.Errorf("web.PreferredPort = %d, want 3000", web.PreferredPort)
	}
	if web.EnvVar != "PORT" {
		t.Errorf("web.EnvVar = %q, want %q", web.EnvVar, "PORT")
	}
	if len(web.EnvFiles) != 1 || web.EnvFiles[0] != ".env" {
		t.Errorf("web.EnvFiles = %v, want [.env]", web.EnvFiles)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	dir := t.TempDir()
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for missing config, got nil")
	}
}

func TestLoad_MissingName(t *testing.T) {
	dir := writeConfig(t, `services:
  web:
    preferred_port: 3000
    env_var: PORT
`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for missing name, got nil")
	}
}

func TestLoad_NoServices(t *testing.T) {
	dir := writeConfig(t, `name: myapp
`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for no services, got nil")
	}
}

func TestLoad_WithProtocol(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    preferred_port: 3000
    env_var: PORT
    protocol: http
  postgres:
    preferred_port: 5432
    env_var: DATABASE_PORT
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Services["web"].Protocol != "http" {
		t.Errorf("web.Protocol = %q, want %q", cfg.Services["web"].Protocol, "http")
	}
	if cfg.Services["postgres"].Protocol != "" {
		t.Errorf("postgres.Protocol = %q, want empty", cfg.Services["postgres"].Protocol)
	}
}

func TestLoad_WithEnvFile(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    preferred_port: 3000
    env_var: PORT
  rails:
    preferred_port: 3000
    env_var: RAILS_PORT
    env_file: backend/.env
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Services["web"].EnvFiles) != 1 || cfg.Services["web"].EnvFiles[0] != ".env" {
		t.Errorf("web.EnvFiles = %v, want [.env]", cfg.Services["web"].EnvFiles)
	}
	if len(cfg.Services["rails"].EnvFiles) != 1 || cfg.Services["rails"].EnvFiles[0] != "backend/.env" {
		t.Errorf("rails.EnvFiles = %v, want [backend/.env]", cfg.Services["rails"].EnvFiles)
	}
}

func TestLoad_WithEnvFileArray(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  postgres:
    preferred_port: 5432
    env_var: DB_PORT
    env_file:
      - backend/.env
      - .env
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pg := cfg.Services["postgres"]
	if len(pg.EnvFiles) != 2 {
		t.Fatalf("postgres.EnvFiles count = %d, want 2", len(pg.EnvFiles))
	}
	if pg.EnvFiles[0] != "backend/.env" || pg.EnvFiles[1] != ".env" {
		t.Errorf("postgres.EnvFiles = %v, want [backend/.env, .env]", pg.EnvFiles)
	}
}

func TestLoad_WithGroups(t *testing.T) {
	dir := writeConfig(t, `name: unio
groups:
  backend:
    env_file: backend/.env
    services:
      rails:
        preferred_port: 3000
        env_var: RAILS_PORT
      postgres:
        preferred_port: 5432
        env_var: DB_PORT
  frontend:
    services:
      main:
        preferred_port: 9000
        env_var: MAIN_PORT
      portal:
        preferred_port: 9001
        env_var: PORTAL_PORT
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Services) != 4 {
		t.Fatalf("services count = %d, want 4", len(cfg.Services))
	}
	if cfg.Services["rails"].EnvFiles[0] != "backend/.env" {
		t.Errorf("rails.EnvFiles = %v, want [backend/.env]", cfg.Services["rails"].EnvFiles)
	}
	if cfg.Services["main"].EnvFiles[0] != ".env" {
		t.Errorf("main.EnvFiles = %v, want [.env]", cfg.Services["main"].EnvFiles)
	}
	if cfg.Services["rails"].Group != "backend" {
		t.Errorf("rails.Group = %q, want %q", cfg.Services["rails"].Group, "backend")
	}
	if cfg.Services["main"].Group != "frontend" {
		t.Errorf("main.Group = %q, want %q", cfg.Services["main"].Group, "frontend")
	}
}

func TestLoad_MixedServicesAndGroups(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  lookbook:
    preferred_port: 4100
    env_var: LOOKBOOK_PORT
    protocol: http
groups:
  backend:
    env_file: backend/.env
    services:
      rails:
        preferred_port: 3000
        env_var: RAILS_PORT
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Services) != 2 {
		t.Fatalf("services count = %d, want 2", len(cfg.Services))
	}
	if cfg.Services["lookbook"].Group != "" {
		t.Errorf("lookbook.Group = %q, want empty", cfg.Services["lookbook"].Group)
	}
	if cfg.Services["rails"].Group != "backend" {
		t.Errorf("rails.Group = %q, want %q", cfg.Services["rails"].Group, "backend")
	}
}

func TestLoad_PerServiceEnvFileOverridesGroup(t *testing.T) {
	dir := writeConfig(t, `name: myapp
groups:
  backend:
    env_file: backend/.env
    services:
      rails:
        preferred_port: 3000
        env_var: RAILS_PORT
      special:
        preferred_port: 4000
        env_var: SPECIAL_PORT
        env_file: special/.env
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Services["rails"].EnvFiles[0] != "backend/.env" {
		t.Errorf("rails.EnvFiles[0] = %q, want %q", cfg.Services["rails"].EnvFiles[0], "backend/.env")
	}
	if cfg.Services["special"].EnvFiles[0] != "special/.env" {
		t.Errorf("special.EnvFiles[0] = %q, want %q", cfg.Services["special"].EnvFiles[0], "special/.env")
	}
}

func TestLoad_DuplicateServiceName(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    preferred_port: 3000
    env_var: PORT
groups:
  frontend:
    services:
      web:
        preferred_port: 4000
        env_var: WEB_PORT
`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for duplicate service name, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "duplicate") {
		t.Errorf("error = %q, want to contain 'duplicate'", err.Error())
	}
}

func TestLoad_MissingEnvVar(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    preferred_port: 3000
`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for missing env_var, got nil")
	}
	if !strings.Contains(err.Error(), "env_var") {
		t.Errorf("error = %q, want to contain 'env_var'", err.Error())
	}
}

func TestLoad_EnvVarCollisionSameFile(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    preferred_port: 3000
    env_var: PORT
  api:
    preferred_port: 4000
    env_var: PORT
`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for env_var collision, got nil")
	}
	if !strings.Contains(err.Error(), "PORT") {
		t.Errorf("error = %q, want to contain 'PORT'", err.Error())
	}
}

func TestLoad_EnvVarSameNameDifferentFiles(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    preferred_port: 3000
    env_var: PORT
  api:
    preferred_port: 4000
    env_var: PORT
    env_file: backend/.env
`)
	_, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v (same env_var in different files should be allowed)", err)
	}
}

func TestLoad_NoPreferredPort(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    env_var: PORT
    protocol: http
  postgres:
    env_var: DATABASE_PORT
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Services) != 2 {
		t.Fatalf("services count = %d, want 2", len(cfg.Services))
	}
	if cfg.Services["web"].PreferredPort != 0 {
		t.Errorf("web.PreferredPort = %d, want 0", cfg.Services["web"].PreferredPort)
	}
	if cfg.Services["web"].EnvVar != "PORT" {
		t.Errorf("web.EnvVar = %q, want %q", cfg.Services["web"].EnvVar, "PORT")
	}
	if cfg.Services["web"].Protocol != "http" {
		t.Errorf("web.Protocol = %q, want %q", cfg.Services["web"].Protocol, "http")
	}
}

func TestLoad_EmptyGroup(t *testing.T) {
	dir := writeConfig(t, `name: myapp
groups:
  empty:
    env_file: backend/.env
services:
  web:
    preferred_port: 3000
    env_var: PORT
`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for empty group, got nil")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("error = %q, want to contain 'empty'", err.Error())
	}
}
