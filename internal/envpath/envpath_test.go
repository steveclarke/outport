package envpath

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClassifyEnvFiles(t *testing.T) {
	projectDir := t.TempDir()

	subdir := filepath.Join(projectDir, "backend")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatal(err)
	}

	externalDir := t.TempDir()

	tests := []struct {
		name     string
		envFiles []string
		wantExt  map[string]bool
	}{
		{
			name:     "dotenv in project root is internal",
			envFiles: []string{".env"},
			wantExt:  map[string]bool{".env": false},
		},
		{
			name:     "dotenv in subdirectory is internal",
			envFiles: []string{"backend/.env"},
			wantExt:  map[string]bool{"backend/.env": false},
		},
		{
			name:     "relative path escaping project is external",
			envFiles: []string{"../" + filepath.Base(externalDir) + "/.env"},
			wantExt:  map[string]bool{"../" + filepath.Base(externalDir) + "/.env": true},
		},
		{
			name:     "absolute path outside project is external",
			envFiles: []string{filepath.Join(externalDir, ".env")},
			wantExt:  map[string]bool{filepath.Join(externalDir, ".env"): true},
		},
		{
			name:     "mixed internal and external",
			envFiles: []string{".env", "../" + filepath.Base(externalDir) + "/.env"},
			wantExt: map[string]bool{
				".env": false,
				"../" + filepath.Base(externalDir) + "/.env": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths, err := ClassifyEnvFiles(projectDir, tt.envFiles)
			if err != nil {
				t.Fatalf("ClassifyEnvFiles: %v", err)
			}
			if len(paths) != len(tt.envFiles) {
				t.Fatalf("got %d paths, want %d", len(paths), len(tt.envFiles))
			}
			for _, p := range paths {
				wantExternal, ok := tt.wantExt[p.ConfigPath]
				if !ok {
					t.Errorf("unexpected config path %q", p.ConfigPath)
					continue
				}
				if p.External != wantExternal {
					t.Errorf("path %q: External = %v, want %v (resolved: %s)",
						p.ConfigPath, p.External, wantExternal, p.ResolvedPath)
				}
				if !filepath.IsAbs(p.ResolvedPath) {
					t.Errorf("path %q: ResolvedPath %q is not absolute", p.ConfigPath, p.ResolvedPath)
				}
			}
		})
	}
}

func TestClassifyEnvFiles_SymlinkInsidePointingOutside(t *testing.T) {
	projectDir := t.TempDir()
	externalDir := t.TempDir()

	linkPath := filepath.Join(projectDir, "linked")
	if err := os.Symlink(externalDir, linkPath); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	paths, err := ClassifyEnvFiles(projectDir, []string{"linked/.env"})
	if err != nil {
		t.Fatalf("ClassifyEnvFiles: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("got %d paths, want 1", len(paths))
	}
	if !paths[0].External {
		t.Error("symlink inside project pointing outside should be classified as external")
	}
}

func TestClassifyEnvFiles_SymlinkOutsidePointingInside(t *testing.T) {
	projectDir := t.TempDir()
	outsideDir := t.TempDir()

	linkPath := filepath.Join(outsideDir, "linked")
	if err := os.Symlink(projectDir, linkPath); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	paths, err := ClassifyEnvFiles(projectDir, []string{filepath.Join(linkPath, ".env")})
	if err != nil {
		t.Fatalf("ClassifyEnvFiles: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("got %d paths, want 1", len(paths))
	}
	if paths[0].External {
		t.Error("symlink outside project pointing inside should be classified as internal")
	}
}

func TestClassifyEnvFiles_ProjectDirIsSymlink(t *testing.T) {
	realDir := t.TempDir()
	linkParent := t.TempDir()
	linkPath := filepath.Join(linkParent, "project-link")
	if err := os.Symlink(realDir, linkPath); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	paths, err := ClassifyEnvFiles(linkPath, []string{".env"})
	if err != nil {
		t.Fatalf("ClassifyEnvFiles: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("got %d paths, want 1", len(paths))
	}
	if paths[0].External {
		t.Error(".env in symlinked project dir should be internal")
	}
}
