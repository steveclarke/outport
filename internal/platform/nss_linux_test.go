//go:build linux

package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindNSSDatabases(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	t.Run("no databases", func(t *testing.T) {
		dbs := FindNSSDatabases()
		if len(dbs) != 0 {
			t.Errorf("expected 0 databases, got %d", len(dbs))
		}
	})

	t.Run("chrome nssdb", func(t *testing.T) {
		dir := filepath.Join(home, ".pki", "nssdb")
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "cert9.db"), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}

		dbs := FindNSSDatabases()
		found := false
		for _, db := range dbs {
			if db.Description == "Chrome/Chromium" {
				found = true
				if db.Path != dir {
					t.Errorf("expected path %s, got %s", dir, db.Path)
				}
			}
		}
		if !found {
			t.Error("Chrome/Chromium database not found")
		}
	})

	t.Run("firefox profiles", func(t *testing.T) {
		profileDir := filepath.Join(home, ".mozilla", "firefox", "abc123.default")
		if err := os.MkdirAll(profileDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(profileDir, "cert9.db"), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}

		dbs := FindNSSDatabases()
		found := false
		for _, db := range dbs {
			if db.Description == "Firefox" {
				found = true
				if db.Path != profileDir {
					t.Errorf("expected path %s, got %s", profileDir, db.Path)
				}
			}
		}
		if !found {
			t.Error("Firefox database not found")
		}
	})

	t.Run("snap chromium", func(t *testing.T) {
		dir := filepath.Join(home, "snap", "chromium", "current", ".pki", "nssdb")
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "cert9.db"), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}

		dbs := FindNSSDatabases()
		found := false
		for _, db := range dbs {
			if db.Description == "Snap Chromium" {
				found = true
			}
		}
		if !found {
			t.Error("Snap Chromium database not found")
		}
	})

	t.Run("skips dirs without cert9.db", func(t *testing.T) {
		profileDir := filepath.Join(home, ".mozilla", "firefox", "empty.profile")
		if err := os.MkdirAll(profileDir, 0755); err != nil {
			t.Fatal(err)
		}
		// No cert9.db

		dbs := FindNSSDatabases()
		for _, db := range dbs {
			if db.Path == profileDir {
				t.Error("should not include profile directory without cert9.db")
			}
		}
	})
}

func TestHasNSSDB(t *testing.T) {
	dir := t.TempDir()

	if hasNSSDB(dir) {
		t.Error("expected false for empty directory")
	}

	if err := os.WriteFile(filepath.Join(dir, "cert9.db"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if !hasNSSDB(dir) {
		t.Error("expected true for directory with cert9.db")
	}
}

func TestCertutilInstallHint(t *testing.T) {
	hint := CertutilInstallHint()
	if hint == "" {
		t.Error("expected non-empty install hint")
	}
}
