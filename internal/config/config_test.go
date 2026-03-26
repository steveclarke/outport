package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "outport.yml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// --- FindDir ---

func TestFindDir_InProjectRoot(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    env_var: PORT
`)
	found, err := FindDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found != dir {
		t.Errorf("FindDir = %q, want %q", found, dir)
	}
}

func TestFindDir_InSubdirectory(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    env_var: PORT
`)
	subdir := filepath.Join(dir, "backend", "app")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	found, err := FindDir(subdir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found != dir {
		t.Errorf("FindDir = %q, want project root %q", found, dir)
	}
}

func TestFindDir_NotFound(t *testing.T) {
	dir := t.TempDir() // no outport.yml anywhere
	_, err := FindDir(dir)
	if err == nil {
		t.Fatal("expected error when no config found, got nil")
	}
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

// --- Computed Values ---

func TestLoad_WithComputedValues(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  rails:
    env_var: RAILS_PORT
    env_file: backend/.env
  web:
    env_var: WEB_PORT
    env_file: frontend/.env

computed:
  API_URL:
    value: "http://localhost:${rails.port}/api/v1"
    env_file: frontend/.env
  CORS_ORIGINS:
    value: "http://localhost:${web.port},http://localhost:${rails.port}"
    env_file: backend/.env
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Computed) != 2 {
		t.Fatalf("computed count = %d, want 2", len(cfg.Computed))
	}
	apiURL := cfg.Computed["API_URL"]
	if apiURL.Value != "http://localhost:${rails.port}/api/v1" {
		t.Errorf("API_URL.Value = %q, want template string", apiURL.Value)
	}
	if len(apiURL.EnvFiles) != 1 || apiURL.EnvFiles[0] != "frontend/.env" {
		t.Errorf("API_URL.EnvFiles = %v, want [frontend/.env]", apiURL.EnvFiles)
	}
	cors := cfg.Computed["CORS_ORIGINS"]
	if len(cors.EnvFiles) != 1 || cors.EnvFiles[0] != "backend/.env" {
		t.Errorf("CORS_ORIGINS.EnvFiles = %v, want [backend/.env]", cors.EnvFiles)
	}
}

func TestLoad_ComputedEnvFileArray(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  rails:
    env_var: RAILS_PORT
    env_file: backend/.env

computed:
  API_URL:
    value: "http://localhost:${rails.port}/api"
    env_file:
      - frontend/main/.env
      - frontend/portal/.env
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Computed["API_URL"].EnvFiles) != 2 {
		t.Fatalf("EnvFiles count = %d, want 2", len(cfg.Computed["API_URL"].EnvFiles))
	}
}

func TestLoad_ComputedPerFileValues(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  rails:
    env_var: RAILS_PORT
    env_file: backend/.env

computed:
  NUXT_API_BASE_URL:
    env_file:
      - file: frontend/apps/main/.env
        value: "http://localhost:${rails.port}/api/v1"
      - file: frontend/apps/portal/.env
        value: "http://localhost:${rails.port}/portal/api/v1"
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	dv := cfg.Computed["NUXT_API_BASE_URL"]
	if len(dv.EnvFiles) != 2 {
		t.Fatalf("EnvFiles count = %d, want 2", len(dv.EnvFiles))
	}
	if dv.PerFile["frontend/apps/main/.env"] != "http://localhost:${rails.port}/api/v1" {
		t.Errorf("main value = %q", dv.PerFile["frontend/apps/main/.env"])
	}
	if dv.PerFile["frontend/apps/portal/.env"] != "http://localhost:${rails.port}/portal/api/v1" {
		t.Errorf("portal value = %q", dv.PerFile["frontend/apps/portal/.env"])
	}
	if dv.Value != "" {
		t.Errorf("Value = %q, want empty (all per-file)", dv.Value)
	}
}

func TestLoad_ComputedMixedEnvFileEntries(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  rails:
    env_var: RAILS_PORT
    env_file: backend/.env

computed:
  API_URL:
    value: "http://localhost:${rails.port}/api"
    env_file:
      - frontend/shared/.env
      - file: frontend/portal/.env
        value: "http://localhost:${rails.port}/portal/api"
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	dv := cfg.Computed["API_URL"]
	if len(dv.EnvFiles) != 2 {
		t.Fatalf("EnvFiles count = %d, want 2", len(dv.EnvFiles))
	}
	if dv.Value != "http://localhost:${rails.port}/api" {
		t.Errorf("Value = %q, want top-level template", dv.Value)
	}
	if dv.PerFile["frontend/portal/.env"] != "http://localhost:${rails.port}/portal/api" {
		t.Errorf("portal value = %q", dv.PerFile["frontend/portal/.env"])
	}
	if _, ok := dv.PerFile["frontend/shared/.env"]; ok {
		t.Error("shared entry should not have a per-file override")
	}
}

func TestLoad_ComputedPerFileValidatesReferences(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  rails:
    env_var: RAILS_PORT
    env_file: backend/.env

computed:
  API_URL:
    env_file:
      - file: frontend/.env
        value: "http://localhost:${missing.port}/api"
`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for invalid reference in per-file value, got nil")
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Errorf("error = %q, want to contain 'missing'", err.Error())
	}
}

func TestLoad_ComputedPerFileMissingValue(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  rails:
    env_var: RAILS_PORT
    env_file: backend/.env

computed:
  API_URL:
    env_file:
      - frontend/shared/.env
      - file: frontend/portal/.env
        value: "http://localhost:${rails.port}/portal/api"
`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for string entry without top-level value, got nil")
	}
	if !strings.Contains(err.Error(), "value") {
		t.Errorf("error = %q, want to contain 'value'", err.Error())
	}
}

// --- Hostname ---

func TestLoad_WithHostname(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    env_var: PORT
    hostname: myapp.localhost
  postgres:
    env_var: DB_PORT
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Services["web"].Hostname != "myapp.localhost" {
		t.Errorf("web.Hostname = %q, want myapp.localhost", cfg.Services["web"].Hostname)
	}
	if cfg.Services["postgres"].Hostname != "" {
		t.Errorf("postgres.Hostname = %q, want empty", cfg.Services["postgres"].Hostname)
	}
}

func TestLoad_ComputedInvalidReference(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    env_var: PORT

computed:
  API_URL:
    value: "http://localhost:${backend.port}/api"
    env_file: frontend/.env
`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for invalid reference, got nil")
	}
	if !strings.Contains(err.Error(), "backend") {
		t.Errorf("error = %q, want to contain 'backend'", err.Error())
	}
}

func TestLoad_ComputedInvalidField(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    env_var: PORT

computed:
  API_URL:
    value: "http://localhost:${web.bogus}"
    env_file: frontend/.env
`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for invalid field, got nil")
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Errorf("error = %q, want to contain 'bogus'", err.Error())
	}
}

func TestLoad_ComputedNameCollidesWithServiceEnvVar(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    env_var: PORT

computed:
  PORT:
    value: "http://localhost:${web.port}"
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

func TestLoad_ComputedMissingValue(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    env_var: PORT

computed:
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

func TestLoad_ComputedMissingEnvFile(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    env_var: PORT

computed:
  API_URL:
    value: "http://localhost:${web.port}/api"
`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for missing env_file, got nil")
	}
	if !strings.Contains(err.Error(), "env_file") {
		t.Errorf("error = %q, want to contain 'env_file'", err.Error())
	}
}

func TestLoad_NoComputedIsValid(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    env_var: PORT
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Computed) != 0 {
		t.Errorf("expected nil or empty computed, got %v", cfg.Computed)
	}
}

// --- Resolution ---

func TestResolveComputed_SubstitutesVars(t *testing.T) {
	computed := map[string]ComputedValue{
		"API_URL": {
			Value:    "http://localhost:${rails.port}/api/v1",
			EnvFiles: []string{"frontend/.env"},
		},
	}
	vars := map[string]string{"rails.port": "24920", "rails.hostname": "localhost"}

	resolved := ResolveComputed(computed, vars)

	if resolved["API_URL"]["frontend/.env"] != "http://localhost:24920/api/v1" {
		t.Errorf("API_URL = %q, want http://localhost:24920/api/v1", resolved["API_URL"]["frontend/.env"])
	}
}

func TestResolveComputed_HostnameReference(t *testing.T) {
	computed := map[string]ComputedValue{
		"CORS": {
			Value:    "http://${web.hostname}:${web.port}",
			EnvFiles: []string{".env"},
		},
	}
	vars := map[string]string{"web.port": "3000", "web.hostname": "myapp.localhost"}

	resolved := ResolveComputed(computed, vars)

	if resolved["CORS"][".env"] != "http://myapp.localhost:3000" {
		t.Errorf("CORS = %q, want http://myapp.localhost:3000", resolved["CORS"][".env"])
	}
}

func TestResolveComputed_MultipleReferences(t *testing.T) {
	computed := map[string]ComputedValue{
		"CORS": {
			Value:    "http://${web.hostname}:${web.port},http://${api.hostname}:${api.port}",
			EnvFiles: []string{".env"},
		},
	}
	vars := map[string]string{
		"web.port": "14139", "web.hostname": "app.localhost",
		"api.port": "24920", "api.hostname": "localhost",
	}

	resolved := ResolveComputed(computed, vars)

	if resolved["CORS"][".env"] != "http://app.localhost:14139,http://localhost:24920" {
		t.Errorf("CORS = %q, want substituted value", resolved["CORS"][".env"])
	}
}

func TestResolveComputed_NoReferences(t *testing.T) {
	computed := map[string]ComputedValue{
		"STATIC": {
			Value:    "some-static-value",
			EnvFiles: []string{".env"},
		},
	}
	vars := map[string]string{"web.port": "3000"}

	resolved := ResolveComputed(computed, vars)

	if resolved["STATIC"][".env"] != "some-static-value" {
		t.Errorf("STATIC = %q, want some-static-value", resolved["STATIC"][".env"])
	}
}

func TestResolveComputed_PerFileValues(t *testing.T) {
	computed := map[string]ComputedValue{
		"API_URL": {
			EnvFiles: []string{"main/.env", "portal/.env"},
			PerFile: map[string]string{
				"main/.env":   "http://localhost:${rails.port}/api/v1",
				"portal/.env": "http://localhost:${rails.port}/portal/api/v1",
			},
		},
	}
	vars := map[string]string{"rails.port": "3000"}

	resolved := ResolveComputed(computed, vars)

	mainVal := resolved["API_URL"]["main/.env"]
	if mainVal != "http://localhost:3000/api/v1" {
		t.Errorf("main = %q, want http://localhost:3000/api/v1", mainVal)
	}
	portalVal := resolved["API_URL"]["portal/.env"]
	if portalVal != "http://localhost:3000/portal/api/v1" {
		t.Errorf("portal = %q, want http://localhost:3000/portal/api/v1", portalVal)
	}
}

func TestResolveComputed_MixedPerFileAndDefault(t *testing.T) {
	computed := map[string]ComputedValue{
		"API_URL": {
			Value:    "http://localhost:${rails.port}/api",
			EnvFiles: []string{"shared/.env", "portal/.env"},
			PerFile: map[string]string{
				"portal/.env": "http://localhost:${rails.port}/portal/api",
			},
		},
	}
	vars := map[string]string{"rails.port": "3000"}

	resolved := ResolveComputed(computed, vars)

	sharedVal := resolved["API_URL"]["shared/.env"]
	if sharedVal != "http://localhost:3000/api" {
		t.Errorf("shared = %q, want default value", sharedVal)
	}
	portalVal := resolved["API_URL"]["portal/.env"]
	if portalVal != "http://localhost:3000/portal/api" {
		t.Errorf("portal = %q, want per-file value", portalVal)
	}
}

func TestResolveComputed_DefaultValueAllFiles(t *testing.T) {
	computed := map[string]ComputedValue{
		"API_URL": {
			Value:    "http://localhost:${web.port}/api",
			EnvFiles: []string{"a/.env", "b/.env"},
		},
	}
	vars := map[string]string{"web.port": "3000"}

	resolved := ResolveComputed(computed, vars)

	for _, file := range []string{"a/.env", "b/.env"} {
		if resolved["API_URL"][file] != "http://localhost:3000/api" {
			t.Errorf("%s = %q, want default resolved value", file, resolved["API_URL"][file])
		}
	}
}

func TestResolveComputed_HostnameDefaultsToLocalhost(t *testing.T) {
	computed := map[string]ComputedValue{
		"URL": {
			Value:    "http://${web.hostname}:${web.port}",
			EnvFiles: []string{".env"},
		},
	}
	// No hostname set on service — should resolve to "localhost"
	vars := map[string]string{"web.port": "3000", "web.hostname": "localhost"}

	resolved := ResolveComputed(computed, vars)

	if resolved["URL"][".env"] != "http://localhost:3000" {
		t.Errorf("URL = %q, want http://localhost:3000", resolved["URL"][".env"])
	}
}

func TestResolveComputed_EmptyMap(t *testing.T) {
	resolved := ResolveComputed(nil, map[string]string{"web.port": "3000"})
	if len(resolved) != 0 {
		t.Errorf("expected empty map, got %v", resolved)
	}
}

// --- Hostname Validation ---

func TestHostnameValidCharacters(t *testing.T) {
	tests := []struct {
		hostname string
		wantErr  bool
	}{
		{"myapp", false},
		{"portal.myapp", false},
		{"myapp-web", false},
		{"my_app", true},    // underscores invalid in DNS
		{"MY_APP", true},    // uppercase invalid
		{"my app", true},    // spaces invalid
		{"othername", true}, // must contain project name "myapp"
	}
	for _, tt := range tests {
		yaml := fmt.Sprintf(`
name: myapp
services:
  web:
    env_var: PORT
    hostname: %s
`, tt.hostname)
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "outport.yml"), []byte(yaml), 0644); err != nil {
			t.Fatal(err)
		}
		_, err := Load(dir)
		if (err != nil) != tt.wantErr {
			t.Errorf("hostname %q: err=%v, wantErr=%v", tt.hostname, err, tt.wantErr)
		}
	}
}

func TestValidateRejectsReservedOutportHostname(t *testing.T) {
	yaml := `
name: outport
services:
  web:
    env_var: PORT
    hostname: outport.test
`
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "outport.yml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for reserved hostname")
	}
	if !strings.Contains(err.Error(), "reserved") {
		t.Errorf("error should mention 'reserved', got: %v", err)
	}
}

// --- Template Modifier Support ---

func TestTemplateModifierParsing(t *testing.T) {
	yaml := `
name: myapp
services:
  rails:
    env_var: PORT
    hostname: myapp
computed:
  API_URL:
    value: "${rails.url:direct}/api"
    env_file: .env
`
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "outport.yml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	vars := map[string]string{
		"rails.port":       "24920",
		"rails.hostname":   "myapp.test",
		"rails.url":        "http://myapp.test",
		"rails.url:direct": "http://localhost:24920",
	}
	resolved := ResolveComputed(cfg.Computed, vars)
	val := resolved["API_URL"][".env"]
	if val != "http://localhost:24920/api" {
		t.Errorf("got %q, want %q", val, "http://localhost:24920/api")
	}
}

func TestTemplateModifierValidation(t *testing.T) {
	yaml := `
name: myapp
services:
  rails:
    env_var: PORT
    hostname: myapp
computed:
  BAD:
    value: "${rails.url:bogus}"
    env_file: .env
`
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "outport.yml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for unrecognized modifier")
	}
}

func TestURLFieldValidation(t *testing.T) {
	yaml := `
name: myapp
services:
  rails:
    env_var: PORT
    hostname: myapp
computed:
  SITE_URL:
    value: "${rails.url}"
    env_file: .env
`
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "outport.yml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(dir)
	if err != nil {
		t.Fatalf("expected no error for url field, got: %v", err)
	}
}

func TestLoad_ComputedProjectNameVariable(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    env_var: PORT

computed:
  COMPOSE_PROJECT_NAME:
    value: "${project_name}${instance:+-${instance}}"
    env_file: .env
`)
	_, err := Load(dir)
	if err != nil {
		t.Fatalf("expected project_name to be a valid variable, got: %v", err)
	}
}

func TestResolveComputed_ProjectName(t *testing.T) {
	computed := map[string]ComputedValue{
		"COMPOSE_PROJECT_NAME": {
			Value:    "${project_name}${instance:+-${instance}}",
			EnvFiles: []string{".env"},
		},
	}

	t.Run("main instance", func(t *testing.T) {
		vars := map[string]string{"project_name": "myapp", "instance": ""}
		resolved := ResolveComputed(computed, vars)
		if got := resolved["COMPOSE_PROJECT_NAME"][".env"]; got != "myapp" {
			t.Errorf("got %q, want %q", got, "myapp")
		}
	})

	t.Run("worktree instance", func(t *testing.T) {
		vars := map[string]string{"project_name": "myapp", "instance": "bxcf"}
		resolved := ResolveComputed(computed, vars)
		if got := resolved["COMPOSE_PROJECT_NAME"][".env"]; got != "myapp-bxcf" {
			t.Errorf("got %q, want %q", got, "myapp-bxcf")
		}
	})
}

func TestLoad_ComputedEnvVarField(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    env_var: PORT

computed:
  WEB_VAR_NAME:
    value: "${web.env_var}"
    env_file: .env
`)
	_, err := Load(dir)
	if err != nil {
		t.Fatalf("expected env_var to be a valid field, got: %v", err)
	}
}

func TestResolveComputed_EnvVarField(t *testing.T) {
	computed := map[string]ComputedValue{
		"VAR_NAME": {
			Value:    "${web.env_var}",
			EnvFiles: []string{".env"},
		},
	}
	vars := map[string]string{"web.env_var": "PORT", "web.port": "3000"}
	resolved := ResolveComputed(computed, vars)
	if got := resolved["VAR_NAME"][".env"]; got != "PORT" {
		t.Errorf("got %q, want %q", got, "PORT")
	}
}

func TestLoad_NoPreferredPort(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    env_var: PORT
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
}

// --- Aliases ---

func TestLoad_ServiceAliases(t *testing.T) {
	dir := writeConfig(t, `name: approvethis
services:
  web:
    env_var: PORT
    hostname: approvethis
    aliases:
      app: app.approvethis
      admin: admin.approvethis
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	web := cfg.Services["web"]
	if len(web.Aliases) != 2 {
		t.Fatalf("aliases count = %d, want 2", len(web.Aliases))
	}
	if web.Aliases["app"] != "app.approvethis" {
		t.Errorf("alias app = %q, want %q", web.Aliases["app"], "app.approvethis")
	}
	if web.Aliases["admin"] != "admin.approvethis" {
		t.Errorf("alias admin = %q, want %q", web.Aliases["admin"], "admin.approvethis")
	}
}

func TestLoad_AliasesWithTestSuffix(t *testing.T) {
	dir := writeConfig(t, `name: approvethis
services:
  web:
    env_var: PORT
    hostname: approvethis.test
    aliases:
      app: app.approvethis.test
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Services["web"].Aliases["app"] != "app.approvethis.test" {
		t.Errorf("alias should preserve .test suffix from config")
	}
}

func TestLoad_NoAliases(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    env_var: PORT
    hostname: myapp
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Services["web"].Aliases != nil {
		t.Errorf("aliases should be nil when not specified")
	}
}

func TestValidateAliasKeyFormat(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{"valid lowercase", "app", false},
		{"valid with hyphens", "my-app", false},
		{"valid with digits", "app2", false},
		{"invalid uppercase", "App", true},
		{"invalid underscore", "my_app", true},
		{"invalid dot", "my.app", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yaml := fmt.Sprintf(`
name: approvethis
services:
  web:
    env_var: PORT
    hostname: approvethis
    aliases:
      %s: app.approvethis
`, tt.key)
			dir := writeConfig(t, yaml)
			_, err := Load(dir)
			if (err != nil) != tt.wantErr {
				t.Errorf("alias key %q: err=%v, wantErr=%v", tt.key, err, tt.wantErr)
			}
		})
	}
}

func TestValidateAliasHostnameRules(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		wantErr  bool
	}{
		{"valid subdomain", "app.approvethis", false},
		{"valid with test suffix", "app.approvethis.test", false},
		{"invalid chars", "app_approvethis", true},
		{"missing project name", "app.other", true},
		{"reserved outport.test", "outport.test", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yaml := fmt.Sprintf(`
name: approvethis
services:
  web:
    env_var: PORT
    hostname: approvethis
    aliases:
      app: %s
`, tt.hostname)
			dir := writeConfig(t, yaml)
			_, err := Load(dir)
			if (err != nil) != tt.wantErr {
				t.Errorf("alias hostname %q: err=%v, wantErr=%v", tt.hostname, err, tt.wantErr)
			}
		})
	}
}

func TestValidateAliasDuplicatesOwnHostname(t *testing.T) {
	dir := writeConfig(t, `
name: approvethis
services:
  web:
    env_var: PORT
    hostname: approvethis
    aliases:
      dupe: approvethis
`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error when alias duplicates primary hostname")
	}
	if !strings.Contains(err.Error(), "conflicts with service's own hostname") {
		t.Errorf("error = %v, want mention of own hostname conflict", err)
	}
}

func TestValidateAliasDuplicatesAnotherAlias(t *testing.T) {
	dir := writeConfig(t, `
name: approvethis
services:
  web:
    env_var: PORT
    hostname: approvethis
    aliases:
      app: app.approvethis
      dupe: app.approvethis
`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error when two aliases share a hostname")
	}
}

func TestValidateAliasWithoutPrimaryHostname(t *testing.T) {
	dir := writeConfig(t, `
name: approvethis
services:
  web:
    env_var: PORT
    aliases:
      app: app.approvethis
`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error when aliases defined without primary hostname")
	}
}

func TestValidateAliasDuplicatesAnotherServiceHostname(t *testing.T) {
	dir := writeConfig(t, `
name: approvethis
services:
  web:
    env_var: PORT
    hostname: approvethis
    aliases:
      app: api.approvethis
  api:
    env_var: API_PORT
    hostname: api.approvethis
`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error when alias duplicates another service's hostname")
	}
}

func TestValidateAliasDuplicatesAnotherServiceAlias(t *testing.T) {
	dir := writeConfig(t, `
name: approvethis
services:
  web:
    env_var: PORT
    hostname: approvethis
    aliases:
      app: app.approvethis
  api:
    env_var: API_PORT
    hostname: api.approvethis
    aliases:
      dupe: app.approvethis
`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error when aliases across services share a hostname")
	}
}

// --- Alias Template References ---

func TestValidateTemplateRefAlias(t *testing.T) {
	dir := writeConfig(t, `name: approvethis
services:
  web:
    env_var: PORT
    hostname: approvethis
    aliases:
      app: app.approvethis
computed:
  APP_URL:
    value: "${web.alias_url.app}"
    env_file: .env
`)
	_, err := Load(dir)
	if err != nil {
		t.Fatalf("expected valid alias template ref, got: %v", err)
	}
}

func TestValidateTemplateRefAliasHostname(t *testing.T) {
	dir := writeConfig(t, `name: approvethis
services:
  web:
    env_var: PORT
    hostname: approvethis
    aliases:
      app: app.approvethis
computed:
  APP_HOSTNAME:
    value: "${web.alias.app}"
    env_file: .env
`)
	_, err := Load(dir)
	if err != nil {
		t.Fatalf("expected valid alias hostname template ref, got: %v", err)
	}
}

func TestValidateTemplateRefAliasUnknown(t *testing.T) {
	dir := writeConfig(t, `name: approvethis
services:
  web:
    env_var: PORT
    hostname: approvethis
    aliases:
      app: app.approvethis
computed:
  BAD:
    value: "${web.alias.missing}"
    env_file: .env
`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for unknown alias name in template")
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Errorf("error = %v, want mention of missing alias", err)
	}
}

// --- Local Config Overrides ---

func writeLocalConfig(t *testing.T, dir string, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, LocalFileName), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestLoad_LocalOverridesPreferredPort(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  postgres:
    env_var: DB_PORT
`)
	writeLocalConfig(t, dir, `services:
  postgres:
    preferred_port: 5432
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Services["postgres"].PreferredPort != 5432 {
		t.Errorf("postgres.PreferredPort = %d, want 5432", cfg.Services["postgres"].PreferredPort)
	}
	// env_var should still come from base config
	if cfg.Services["postgres"].EnvVar != "DB_PORT" {
		t.Errorf("postgres.EnvVar = %q, want DB_PORT", cfg.Services["postgres"].EnvVar)
	}
}

func TestLoad_LocalOverridesEnvVar(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    env_var: PORT
`)
	writeLocalConfig(t, dir, `services:
  web:
    env_var: CUSTOM_PORT
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Services["web"].EnvVar != "CUSTOM_PORT" {
		t.Errorf("web.EnvVar = %q, want CUSTOM_PORT", cfg.Services["web"].EnvVar)
	}
}

func TestLoad_LocalOverridesHostname(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    env_var: PORT
    hostname: myapp
`)
	writeLocalConfig(t, dir, `services:
  web:
    hostname: dev.myapp
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Services["web"].Hostname != "dev.myapp" {
		t.Errorf("web.Hostname = %q, want dev.myapp", cfg.Services["web"].Hostname)
	}
	// env_var should still come from base config
	if cfg.Services["web"].EnvVar != "PORT" {
		t.Errorf("web.EnvVar = %q, want PORT", cfg.Services["web"].EnvVar)
	}
}

func TestLoad_LocalOverridesEnvFile(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    env_var: PORT
    env_file: .env
`)
	writeLocalConfig(t, dir, `services:
  web:
    env_file: backend/.env
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Services["web"].EnvFiles) != 1 || cfg.Services["web"].EnvFiles[0] != "backend/.env" {
		t.Errorf("web.EnvFiles = %v, want [backend/.env]", cfg.Services["web"].EnvFiles)
	}
}

func TestLoad_LocalOverridesAliases(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    env_var: PORT
    hostname: myapp
    aliases:
      app: app.myapp
`)
	writeLocalConfig(t, dir, `services:
  web:
    aliases:
      admin: admin.myapp
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Local aliases replace base aliases entirely (map replacement, not merge)
	if _, ok := cfg.Services["web"].Aliases["admin"]; !ok {
		t.Error("expected admin alias from local override")
	}
	if _, ok := cfg.Services["web"].Aliases["app"]; ok {
		t.Error("expected base alias 'app' to be replaced by local override")
	}
}

func TestValidateTemplateRefAliasUnknownService(t *testing.T) {
	dir := writeConfig(t, `name: approvethis
services:
  web:
    env_var: PORT
    hostname: approvethis
computed:
  BAD:
    value: "${unknown.alias.app}"
    env_file: .env
`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for unknown service in alias template ref")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Errorf("error = %v, want mention of unknown service", err)
	}
}

func TestLoad_LocalUnknownServiceErrors(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    env_var: PORT
`)
	writeLocalConfig(t, dir, `services:
  storybook:
    preferred_port: 6006
`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for unknown service in local config")
	}
	if !strings.Contains(err.Error(), "storybook") {
		t.Errorf("error = %q, want to contain 'storybook'", err.Error())
	}
	if !strings.Contains(err.Error(), LocalFileName) {
		t.Errorf("error = %q, want to contain %q", err.Error(), LocalFileName)
	}
}

func TestLoad_LocalInvalidYAMLErrors(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    env_var: PORT
`)
	writeLocalConfig(t, dir, `{{{ not valid yaml`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for invalid YAML in local config")
	}
	if !strings.Contains(err.Error(), LocalFileName) {
		t.Errorf("error = %q, want to contain %q", err.Error(), LocalFileName)
	}
}

func TestLoad_NoLocalFileIsValid(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    env_var: PORT
`)
	// No outport.local.yml — should work fine
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Services["web"].EnvVar != "PORT" {
		t.Errorf("web.EnvVar = %q, want PORT", cfg.Services["web"].EnvVar)
	}
}

func TestLoad_EmptyLocalFileIsValid(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    env_var: PORT
`)
	writeLocalConfig(t, dir, ``)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Services["web"].EnvVar != "PORT" {
		t.Errorf("web.EnvVar = %q, want PORT", cfg.Services["web"].EnvVar)
	}
}

func TestLoad_LocalOverrideMergedThenValidated(t *testing.T) {
	// Local override produces an invalid hostname — should fail validation
	dir := writeConfig(t, `name: myapp
services:
  web:
    env_var: PORT
    hostname: myapp
`)
	writeLocalConfig(t, dir, `services:
  web:
    hostname: INVALID_HOST
`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected validation error for invalid hostname from local override")
	}
	if !strings.Contains(err.Error(), "invalid characters") {
		t.Errorf("error = %q, want hostname validation error", err.Error())
	}
}

func TestLoad_LocalOverridesSubsetOfServices(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    env_var: PORT
  postgres:
    env_var: DB_PORT
  redis:
    env_var: REDIS_PORT
`)
	// Only override postgres — other services unchanged
	writeLocalConfig(t, dir, `services:
  postgres:
    preferred_port: 5432
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Services["postgres"].PreferredPort != 5432 {
		t.Errorf("postgres.PreferredPort = %d, want 5432", cfg.Services["postgres"].PreferredPort)
	}
	if cfg.Services["web"].PreferredPort != 0 {
		t.Errorf("web.PreferredPort = %d, want 0 (unchanged)", cfg.Services["web"].PreferredPort)
	}
	if cfg.Services["redis"].PreferredPort != 0 {
		t.Errorf("redis.PreferredPort = %d, want 0 (unchanged)", cfg.Services["redis"].PreferredPort)
	}
}

// --- Open field ---

func TestLoad_OpenField(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    env_var: PORT
    hostname: myapp
  admin:
    env_var: ADMIN_PORT
    hostname: admin.myapp
  postgres:
    env_var: DB_PORT
open:
  - web
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Open) != 1 || cfg.Open[0] != "web" {
		t.Errorf("Open = %v, want [web]", cfg.Open)
	}
}

func TestLoad_OpenFieldAbsent(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    env_var: PORT
    hostname: myapp
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Open != nil {
		t.Errorf("Open = %v, want nil", cfg.Open)
	}
}

func TestLoad_OpenUnknownServiceErrors(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    env_var: PORT
    hostname: myapp
open:
  - web
  - missing
`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for unknown service in open, got nil")
	}
	if !strings.Contains(err.Error(), `"missing"`) {
		t.Errorf("error = %q, want to contain '\"missing\"'", err.Error())
	}
}

func TestLoad_OpenServiceWithoutHostnameErrors(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    env_var: PORT
    hostname: myapp
  postgres:
    env_var: DB_PORT
open:
  - web
  - postgres
`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for service without hostname in open, got nil")
	}
	if !strings.Contains(err.Error(), `"postgres"`) {
		t.Errorf("error = %q, want to contain '\"postgres\"'", err.Error())
	}
	if !strings.Contains(err.Error(), "hostname") {
		t.Errorf("error = %q, want to contain 'hostname'", err.Error())
	}
}

func TestLoad_OpenDuplicateErrors(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    env_var: PORT
    hostname: myapp
open:
  - web
  - web
`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for duplicate in open, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("error = %q, want to contain 'duplicate'", err.Error())
	}
}

