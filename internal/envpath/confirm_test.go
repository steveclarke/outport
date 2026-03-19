package envpath

import (
	"bytes"
	"strings"
	"testing"
)

func TestConfirmExternalFiles_NoExternalPaths(t *testing.T) {
	paths := []EnvFilePath{
		{ConfigPath: ".env", ResolvedPath: "/project/.env", External: false},
	}
	approved, err := ConfirmExternalFiles(paths, nil, "/project", false, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(approved) != 0 {
		t.Errorf("expected no newly approved, got %v", approved)
	}
}

func TestConfirmExternalFiles_AutoApprove(t *testing.T) {
	paths := []EnvFilePath{
		{ConfigPath: "../other/.env", ResolvedPath: "/other/.env", External: true},
	}
	stderr := new(bytes.Buffer)
	approved, err := ConfirmExternalFiles(paths, nil, "/project", true, nil, stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(approved) != 1 || approved[0] != "/other/.env" {
		t.Errorf("expected [/other/.env], got %v", approved)
	}
}

func TestConfirmExternalFiles_AlreadyApproved(t *testing.T) {
	paths := []EnvFilePath{
		{ConfigPath: "../other/.env", ResolvedPath: "/other/.env", External: true},
	}
	approved, err := ConfirmExternalFiles(paths, []string{"/other/.env"}, "/project", false, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(approved) != 0 {
		t.Errorf("expected no newly approved, got %v", approved)
	}
}

func TestConfirmExternalFiles_NonInteractiveReturnsError(t *testing.T) {
	paths := []EnvFilePath{
		{ConfigPath: "../other/.env", ResolvedPath: "/other/.env", External: true},
	}
	stdin := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	_, err := ConfirmExternalFiles(paths, nil, "/project", false, stdin, stderr)
	if err == nil {
		t.Fatal("expected error for non-interactive stdin with unapproved paths")
	}
	if !strings.Contains(err.Error(), "-y") {
		t.Errorf("error should mention -y flag, got: %v", err)
	}
}

func TestConfirmExternalFiles_MixApprovedAndNew(t *testing.T) {
	paths := []EnvFilePath{
		{ConfigPath: "../old/.env", ResolvedPath: "/old/.env", External: true},
		{ConfigPath: "../new/.env", ResolvedPath: "/new/.env", External: true},
	}
	approved, err := ConfirmExternalFiles(paths, []string{"/old/.env"}, "/project", true, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(approved) != 1 || approved[0] != "/new/.env" {
		t.Errorf("expected [/new/.env], got %v", approved)
	}
}
