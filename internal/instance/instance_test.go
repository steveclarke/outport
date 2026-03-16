package instance

import (
	"strings"
	"testing"

	"github.com/outport-app/outport/internal/registry"
)

func buildTestRegistry(projects map[string]struct{ dir string }) *registry.Registry {
	reg := &registry.Registry{Projects: make(map[string]registry.Allocation)}
	for key, p := range projects {
		parts := strings.SplitN(key, "/", 2)
		reg.Set(parts[0], parts[1], registry.Allocation{ProjectDir: p.dir})
	}
	return reg
}

func TestResolveExistingInstance(t *testing.T) {
	projects := map[string]struct{ dir string }{
		"myapp/main": {dir: "/src/myapp"},
		"myapp/bkrm": {dir: "/tmp/myapp-clone"},
	}
	reg := buildTestRegistry(projects)

	name, isNew, err := Resolve(reg, "myapp", "/src/myapp")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if name != "main" {
		t.Errorf("name: got %q, want %q", name, "main")
	}
	if isNew {
		t.Error("expected isNew=false for existing instance")
	}
}

func TestResolveFirstInstance(t *testing.T) {
	reg := buildTestRegistry(nil)

	name, isNew, err := Resolve(reg, "myapp", "/src/myapp")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if name != "main" {
		t.Errorf("name: got %q, want %q", name, "main")
	}
	if !isNew {
		t.Error("expected isNew=true for first instance")
	}
}

func TestResolveSubsequentInstance(t *testing.T) {
	projects := map[string]struct{ dir string }{
		"myapp/main": {dir: "/src/myapp"},
	}
	reg := buildTestRegistry(projects)

	name, isNew, err := Resolve(reg, "myapp", "/tmp/myapp-clone")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if name == "main" {
		t.Error("subsequent instance should not be 'main'")
	}
	if len(name) != 4 {
		t.Errorf("expected 4-char code, got %q", name)
	}
	if !isNew {
		t.Error("expected isNew=true for new instance")
	}
}

func TestGenerateCode(t *testing.T) {
	used := map[string]bool{}
	code := GenerateCode(used)
	if len(code) != 4 {
		t.Fatalf("code length: got %d, want 4", len(code))
	}
	for _, c := range code {
		if !isConsonant(byte(c)) {
			t.Errorf("code %q contains non-consonant %c", code, c)
		}
	}
}

func TestGenerateCodeAvoidsCollisions(t *testing.T) {
	used := map[string]bool{}
	codes := make(map[string]bool)
	for i := 0; i < 100; i++ {
		code := GenerateCode(used)
		if codes[code] {
			t.Fatalf("duplicate code %q on iteration %d", code, i)
		}
		codes[code] = true
		used[code] = true
	}
}

func TestValidateInstanceName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"main", false},
		{"feature-xyz", false},
		{"bkrm", false},
		{"agent-1", false},
		{"", true},
		{"UPPER", true},
		{"has space", true},
		{"has_underscore", true},
	}
	for _, tt := range tests {
		err := ValidateName(tt.name)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateName(%q): err=%v, wantErr=%v", tt.name, err, tt.wantErr)
		}
	}
}
