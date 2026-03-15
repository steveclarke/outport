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

// --- Derived Values ---

func TestLoad_WithDerivedValues(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  rails:
    env_var: RAILS_PORT
    protocol: http
    env_file: backend/.env
  web:
    env_var: WEB_PORT
    protocol: http
    env_file: frontend/.env

derived:
  API_URL:
    value: "http://localhost:${RAILS_PORT}/api/v1"
    env_file: frontend/.env
  CORS_ORIGINS:
    value: "http://localhost:${WEB_PORT},http://localhost:${RAILS_PORT}"
    env_file: backend/.env
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Derived) != 2 {
		t.Fatalf("derived count = %d, want 2", len(cfg.Derived))
	}
	apiURL := cfg.Derived["API_URL"]
	if apiURL.Value != "http://localhost:${RAILS_PORT}/api/v1" {
		t.Errorf("API_URL.Value = %q, want template string", apiURL.Value)
	}
	if len(apiURL.EnvFiles) != 1 || apiURL.EnvFiles[0] != "frontend/.env" {
		t.Errorf("API_URL.EnvFiles = %v, want [frontend/.env]", apiURL.EnvFiles)
	}
	cors := cfg.Derived["CORS_ORIGINS"]
	if len(cors.EnvFiles) != 1 || cors.EnvFiles[0] != "backend/.env" {
		t.Errorf("CORS_ORIGINS.EnvFiles = %v, want [backend/.env]", cors.EnvFiles)
	}
}

func TestLoad_DerivedEnvFileArray(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  rails:
    env_var: RAILS_PORT
    env_file: backend/.env

derived:
  API_URL:
    value: "http://localhost:${RAILS_PORT}/api"
    env_file:
      - frontend/main/.env
      - frontend/portal/.env
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Derived["API_URL"].EnvFiles) != 2 {
		t.Fatalf("EnvFiles count = %d, want 2", len(cfg.Derived["API_URL"].EnvFiles))
	}
}

func TestLoad_DerivedInvalidReference(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    env_var: PORT

derived:
  API_URL:
    value: "http://localhost:${BACKEND_PORT}/api"
    env_file: frontend/.env
`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for invalid reference, got nil")
	}
	if !strings.Contains(err.Error(), "BACKEND_PORT") {
		t.Errorf("error = %q, want to contain 'BACKEND_PORT'", err.Error())
	}
}

func TestLoad_DerivedNameCollidesWithServiceEnvVar(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    env_var: PORT

derived:
  PORT:
    value: "http://localhost:${PORT}"
    env_file: frontend/.env
`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for name collision, got nil")
	}
	if !strings.Contains(err.Error(), "PORT") {
		t.Errorf("error = %q, want to contain 'PORT'", err.Error())
	}
}

func TestLoad_DerivedMissingValue(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    env_var: PORT

derived:
  API_URL:
    env_file: frontend/.env
`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for missing value, got nil")
	}
	if !strings.Contains(err.Error(), "value") {
		t.Errorf("error = %q, want to contain 'value'", err.Error())
	}
}

func TestLoad_DerivedMissingEnvFile(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    env_var: PORT

derived:
  API_URL:
    value: "http://localhost:${PORT}/api"
`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for missing env_file, got nil")
	}
	if !strings.Contains(err.Error(), "env_file") {
		t.Errorf("error = %q, want to contain 'env_file'", err.Error())
	}
}

func TestLoad_NoDerivedIsValid(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    env_var: PORT
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Derived != nil && len(cfg.Derived) != 0 {
		t.Errorf("expected nil or empty derived, got %v", cfg.Derived)
	}
}

// --- Resolution ---

func TestResolveDerived_SubstitutesVars(t *testing.T) {
	derived := map[string]DerivedValue{
		"API_URL": {
			Value:    "http://localhost:${RAILS_PORT}/api/v1",
			EnvFiles: []string{"frontend/.env"},
		},
	}
	envVarPorts := map[string]int{"RAILS_PORT": 24920}

	resolved := ResolveDerived(derived, envVarPorts)

	if resolved["API_URL"] != "http://localhost:24920/api/v1" {
		t.Errorf("API_URL = %q, want http://localhost:24920/api/v1", resolved["API_URL"])
	}
}

func TestResolveDerived_MultipleReferences(t *testing.T) {
	derived := map[string]DerivedValue{
		"CORS": {
			Value:    "http://localhost:${WEB_PORT},http://localhost:${API_PORT}",
			EnvFiles: []string{".env"},
		},
	}
	envVarPorts := map[string]int{"WEB_PORT": 14139, "API_PORT": 24920}

	resolved := ResolveDerived(derived, envVarPorts)

	if resolved["CORS"] != "http://localhost:14139,http://localhost:24920" {
		t.Errorf("CORS = %q, want substituted value", resolved["CORS"])
	}
}

func TestResolveDerived_NoReferences(t *testing.T) {
	derived := map[string]DerivedValue{
		"STATIC": {
			Value:    "some-static-value",
			EnvFiles: []string{".env"},
		},
	}
	envVarPorts := map[string]int{"PORT": 3000}

	resolved := ResolveDerived(derived, envVarPorts)

	if resolved["STATIC"] != "some-static-value" {
		t.Errorf("STATIC = %q, want some-static-value", resolved["STATIC"])
	}
}

func TestResolveDerived_EmptyMap(t *testing.T) {
	resolved := ResolveDerived(nil, map[string]int{"PORT": 3000})
	if len(resolved) != 0 {
		t.Errorf("expected empty map, got %v", resolved)
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

