package worktree

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetect_MainCheckout(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	info, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Instance != "main" {
		t.Errorf("Instance = %q, want %q", info.Instance, "main")
	}
	if info.IsWorktree {
		t.Error("IsWorktree = true, want false")
	}
}

func TestDetect_Worktree(t *testing.T) {
	dir := t.TempDir()
	gitContent := "gitdir: /Users/someone/project/.git/worktrees/feature-xyz\n"
	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte(gitContent), 0644); err != nil {
		t.Fatal(err)
	}

	info, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Instance != "feature-xyz" {
		t.Errorf("Instance = %q, want %q", info.Instance, "feature-xyz")
	}
	if !info.IsWorktree {
		t.Error("IsWorktree = false, want true")
	}
}

func TestDetect_NoGit(t *testing.T) {
	dir := t.TempDir()
	info, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Instance != "main" {
		t.Errorf("Instance = %q, want %q", info.Instance, "main")
	}
	if info.IsWorktree {
		t.Error("IsWorktree = true, want false")
	}
}
