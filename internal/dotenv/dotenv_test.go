package dotenv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const beginMarker = "# --- begin outport.dev ---"
const endMarker = "# --- end outport.dev ---"

func readEnv(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading env file: %v", err)
	}
	return string(data)
}

func TestMerge_NewFile(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	ports := map[string]string{
		"PORT":          "31653",
		"DATABASE_PORT": "17842",
	}
	if err := Merge(envPath, ports); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := readEnv(t, envPath)

	if !strings.Contains(content, beginMarker) {
		t.Error("missing begin marker")
	}
	if !strings.Contains(content, endMarker) {
		t.Error("missing end marker")
	}
	if !strings.Contains(content, "DATABASE_PORT=17842") {
		t.Error("missing DATABASE_PORT=17842")
	}
	if !strings.Contains(content, "PORT=31653") {
		t.Error("missing PORT=31653")
	}
	// No inline comments
	if strings.Contains(content, "# managed by outport") {
		t.Error("should not have inline comments")
	}
}

func TestMerge_PreservesUnrelatedVars(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	existing := "SECRET_KEY=abc123\nRAILS_ENV=development\n"
	if err := os.WriteFile(envPath, []byte(existing), 0644); err != nil {
		t.Fatalf("write env: %v", err)
	}

	ports := map[string]string{"PORT": "31653"}
	if err := Merge(envPath, ports); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := readEnv(t, envPath)

	if !strings.Contains(content, "SECRET_KEY=abc123") {
		t.Error("lost existing SECRET_KEY")
	}
	if !strings.Contains(content, "RAILS_ENV=development") {
		t.Error("lost existing RAILS_ENV")
	}
	if !strings.Contains(content, "PORT=31653") {
		t.Error("missing PORT=31653 in managed block")
	}
	// User vars should be ABOVE the block
	beginIdx := strings.Index(content, beginMarker)
	secretIdx := strings.Index(content, "SECRET_KEY=abc123")
	if secretIdx > beginIdx {
		t.Error("user vars should appear before the managed block")
	}
}

func TestMerge_RemovesManagedVarFromUserSection(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	// PORT exists in the user section — should be removed and placed in the block
	existing := "PORT=4000\nSECRET_KEY=abc123\n"
	if err := os.WriteFile(envPath, []byte(existing), 0644); err != nil {
		t.Fatalf("write env: %v", err)
	}

	ports := map[string]string{"PORT": "31653"}
	if err := Merge(envPath, ports); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := readEnv(t, envPath)

	if strings.Contains(content, "PORT=4000") {
		t.Error("old PORT value should be removed from user section")
	}
	if !strings.Contains(content, "SECRET_KEY=abc123") {
		t.Error("lost existing SECRET_KEY")
	}
	if !strings.Contains(content, "PORT=31653") {
		t.Error("missing PORT=31653 in managed block")
	}
}

func TestMerge_PreservesComments(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	existing := "# Database config\nDB_PORT=5432\n\n# Redis\nREDIS_PORT=6379\n"
	if err := os.WriteFile(envPath, []byte(existing), 0644); err != nil {
		t.Fatalf("write env: %v", err)
	}

	ports := map[string]string{"DB_PORT": "21536"}
	if err := Merge(envPath, ports); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := readEnv(t, envPath)

	if !strings.Contains(content, "# Database config") {
		t.Error("lost comment")
	}
	if !strings.Contains(content, "REDIS_PORT=6379") {
		t.Error("lost unrelated REDIS_PORT")
	}
	if !strings.Contains(content, "DB_PORT=21536") {
		t.Error("missing DB_PORT in managed block")
	}
	// Old DB_PORT=5432 should be removed from user section
	if strings.Contains(content, "DB_PORT=5432") {
		t.Error("old DB_PORT should be removed from user section")
	}
}

func TestMerge_PreservesBlankLinesInUserSection(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	existing := "ALPHA=1\n\nBETA=2\n"
	if err := os.WriteFile(envPath, []byte(existing), 0644); err != nil {
		t.Fatalf("write env: %v", err)
	}

	ports := map[string]string{"PORT": "31653"}
	if err := Merge(envPath, ports); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := readEnv(t, envPath)

	// User section should preserve blank lines between vars
	userSection := content[:strings.Index(content, beginMarker)]
	if !strings.Contains(userSection, "ALPHA=1\n\nBETA=2") {
		t.Errorf("blank line in user section was lost, got:\n%s", userSection)
	}
}

func TestMerge_HandlesExportPrefix(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	existing := "export PORT=4000\n"
	if err := os.WriteFile(envPath, []byte(existing), 0644); err != nil {
		t.Fatalf("write env: %v", err)
	}

	ports := map[string]string{"PORT": "31653"}
	if err := Merge(envPath, ports); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := readEnv(t, envPath)

	if strings.Contains(content, "4000") {
		t.Error("old export PORT value should be removed")
	}
	if !strings.Contains(content, "PORT=31653") {
		t.Error("missing PORT in managed block")
	}
}

func TestMerge_HandlesQuotedValues(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	existing := "SECRET=\"my secret\"\nPORT=4000\n"
	if err := os.WriteFile(envPath, []byte(existing), 0644); err != nil {
		t.Fatalf("write env: %v", err)
	}

	ports := map[string]string{"PORT": "31653"}
	if err := Merge(envPath, ports); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := readEnv(t, envPath)

	if !strings.Contains(content, "SECRET=\"my secret\"") {
		t.Error("lost quoted SECRET value")
	}
	if !strings.Contains(content, "PORT=31653") {
		t.Error("missing PORT in managed block")
	}
}

func TestMerge_IsIdempotent(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	ports := map[string]string{"PORT": "31653", "DB_PORT": "17842"}

	if err := Merge(envPath, ports); err != nil {
		t.Fatalf("first merge: %v", err)
	}
	data1, _ := os.ReadFile(envPath)

	if err := Merge(envPath, ports); err != nil {
		t.Fatalf("second merge: %v", err)
	}
	data2, _ := os.ReadFile(envPath)

	if string(data1) != string(data2) {
		t.Errorf("merge is not idempotent:\nfirst:\n%s\nsecond:\n%s", data1, data2)
	}
}

func TestMerge_CommentedOutVarIsNotRemoved(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	existing := "# PORT=4000\n"
	if err := os.WriteFile(envPath, []byte(existing), 0644); err != nil {
		t.Fatalf("write env: %v", err)
	}

	ports := map[string]string{"PORT": "31653"}
	if err := Merge(envPath, ports); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := readEnv(t, envPath)

	if !strings.Contains(content, "# PORT=4000") {
		t.Error("commented line should be preserved")
	}
	if !strings.Contains(content, "PORT=31653") {
		t.Error("missing PORT in managed block")
	}
}

func TestMerge_UpdatesExistingBlock(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	// File already has a managed block from a previous apply
	existing := "SECRET=abc\n\n" + beginMarker + "\nPORT=31653\n" + endMarker + "\n"
	if err := os.WriteFile(envPath, []byte(existing), 0644); err != nil {
		t.Fatalf("write env: %v", err)
	}

	// Apply with different ports
	ports := map[string]string{"PORT": "9999", "DB_PORT": "5432"}
	if err := Merge(envPath, ports); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := readEnv(t, envPath)

	if !strings.Contains(content, "SECRET=abc") {
		t.Error("lost user SECRET")
	}
	if !strings.Contains(content, "PORT=9999") {
		t.Error("PORT should be updated to 9999")
	}
	if !strings.Contains(content, "DB_PORT=5432") {
		t.Error("missing new DB_PORT")
	}
	if strings.Contains(content, "PORT=31653") {
		t.Error("old PORT value should be gone")
	}
	// Should have exactly one begin and one end marker
	if strings.Count(content, beginMarker) != 1 {
		t.Errorf("expected 1 begin marker, got %d", strings.Count(content, beginMarker))
	}
	if strings.Count(content, endMarker) != 1 {
		t.Errorf("expected 1 end marker, got %d", strings.Count(content, endMarker))
	}
}

func TestMerge_MigratesInlineComments(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	// Old format: inline comments from previous outport versions
	existing := "SECRET=abc\nPORT=31653 # managed by outport\nDB_PORT=17842 # managed by outport\n"
	if err := os.WriteFile(envPath, []byte(existing), 0644); err != nil {
		t.Fatalf("write env: %v", err)
	}

	ports := map[string]string{"PORT": "31653", "DB_PORT": "17842"}
	if err := Merge(envPath, ports); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := readEnv(t, envPath)

	// Inline comments should be gone
	if strings.Contains(content, "# managed by outport") {
		t.Error("inline comments should be removed during migration")
	}
	// Values should be in the fenced block
	if !strings.Contains(content, beginMarker) {
		t.Error("missing begin marker")
	}
	if !strings.Contains(content, "PORT=31653") {
		t.Error("missing PORT in block")
	}
	if !strings.Contains(content, "DB_PORT=17842") {
		t.Error("missing DB_PORT in block")
	}
	if !strings.Contains(content, "SECRET=abc") {
		t.Error("lost user SECRET")
	}
}

func TestMerge_BlockVarsSortedAlphabetically(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	ports := map[string]string{"ZEBRA": "1", "ALPHA": "2", "MIDDLE": "3"}
	if err := Merge(envPath, ports); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := readEnv(t, envPath)
	lines := strings.Split(content, "\n")

	// Find the managed lines (between markers)
	var managed []string
	inBlock := false
	for _, line := range lines {
		if line == beginMarker {
			inBlock = true
			continue
		}
		if line == endMarker {
			break
		}
		if inBlock && line != "" {
			managed = append(managed, line)
		}
	}

	if len(managed) != 3 {
		t.Fatalf("expected 3 managed lines, got %d: %v", len(managed), managed)
	}
	if managed[0] != "ALPHA=2" || managed[1] != "MIDDLE=3" || managed[2] != "ZEBRA=1" {
		t.Errorf("managed lines not sorted: %v", managed)
	}
}

func TestMerge_PreservesContentAfterBlock(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	// Someone added content after the block
	existing := "SECRET=abc\n\n" + beginMarker + "\nPORT=3000\n" + endMarker + "\n\n# My custom stuff\nFOO=bar\n"
	if err := os.WriteFile(envPath, []byte(existing), 0644); err != nil {
		t.Fatalf("write env: %v", err)
	}

	ports := map[string]string{"PORT": "9999"}
	if err := Merge(envPath, ports); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := readEnv(t, envPath)

	if !strings.Contains(content, "SECRET=abc") {
		t.Error("lost pre-block content")
	}
	if !strings.Contains(content, "FOO=bar") {
		t.Error("lost post-block content")
	}
	if !strings.Contains(content, "# My custom stuff") {
		t.Error("lost post-block comment")
	}
	if !strings.Contains(content, "PORT=9999") {
		t.Error("PORT should be updated")
	}
}

func TestMerge_RemovedVarsDisappearFromBlock(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	// First apply with two vars
	if err := Merge(envPath, map[string]string{"PORT": "3000", "DB_PORT": "5432"}); err != nil {
		t.Fatalf("first merge: %v", err)
	}

	// Second apply with one var removed
	if err := Merge(envPath, map[string]string{"PORT": "3000"}); err != nil {
		t.Fatalf("second merge: %v", err)
	}

	content := readEnv(t, envPath)

	if !strings.Contains(content, "PORT=3000") {
		t.Error("missing PORT")
	}
	if strings.Contains(content, "DB_PORT") {
		t.Error("DB_PORT should be gone after removal from managed set")
	}
}

func TestMerge_EmptyPortsWritesEmptyBlock(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	existing := "SECRET=abc\n"
	if err := os.WriteFile(envPath, []byte(existing), 0644); err != nil {
		t.Fatalf("write env: %v", err)
	}

	ports := map[string]string{}
	if err := Merge(envPath, ports); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := readEnv(t, envPath)

	if !strings.Contains(content, "SECRET=abc") {
		t.Error("lost user SECRET")
	}
	// With no managed vars, no block should be written
	if strings.Contains(content, beginMarker) {
		t.Error("should not write block when no managed vars")
	}
}
