package dotenv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

	data, _ := os.ReadFile(envPath)
	content := string(data)

	if !strings.Contains(content, "PORT=31653 # managed by outport") {
		t.Error("missing PORT=31653 # managed by outport")
	}
	if !strings.Contains(content, "DATABASE_PORT=17842 # managed by outport") {
		t.Error("missing DATABASE_PORT=17842 # managed by outport")
	}
}

func TestMerge_PreservesUnrelatedVars(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	existing := "SECRET_KEY=abc123\nRAILS_ENV=development\n"
	os.WriteFile(envPath, []byte(existing), 0644)

	ports := map[string]string{"PORT": "31653"}
	if err := Merge(envPath, ports); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(envPath)
	content := string(data)

	if !strings.Contains(content, "SECRET_KEY=abc123") {
		t.Error("lost existing SECRET_KEY")
	}
	if !strings.Contains(content, "RAILS_ENV=development") {
		t.Error("lost existing RAILS_ENV")
	}
	if !strings.Contains(content, "PORT=31653 # managed by outport") {
		t.Error("missing PORT=31653 # managed by outport")
	}
}

func TestMerge_OverwritesExistingVar(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	existing := "PORT=4000\nSECRET_KEY=abc123\n"
	os.WriteFile(envPath, []byte(existing), 0644)

	ports := map[string]string{"PORT": "31653"}
	if err := Merge(envPath, ports); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(envPath)
	content := string(data)

	if strings.Contains(content, "PORT=4000") {
		t.Error("old PORT value should be overwritten")
	}
	if !strings.Contains(content, "PORT=31653 # managed by outport") {
		t.Error("missing updated PORT=31653 # managed by outport")
	}
	if !strings.Contains(content, "SECRET_KEY=abc123") {
		t.Error("lost existing SECRET_KEY")
	}
}

func TestMerge_UpdatesValueInPlace(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	existing := "FIRST=1\nPORT=4000\nLAST=3\n"
	os.WriteFile(envPath, []byte(existing), 0644)

	ports := map[string]string{"PORT": "31653"}
	if err := Merge(envPath, ports); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(envPath)
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")

	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %v", len(lines), lines)
	}
	if lines[1] != "PORT=31653 # managed by outport" {
		t.Errorf("line 2 = %q, want PORT=31653 # managed by outport", lines[1])
	}
}

func TestMerge_PreservesComments(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	existing := "# Database config\nDB_PORT=5432\n\n# Redis\nREDIS_PORT=6379\n"
	os.WriteFile(envPath, []byte(existing), 0644)

	ports := map[string]string{"DB_PORT": "21536"}
	if err := Merge(envPath, ports); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(envPath)
	content := string(data)

	if !strings.Contains(content, "# Database config") {
		t.Error("lost comment")
	}
	if !strings.Contains(content, "DB_PORT=21536 # managed by outport") {
		t.Error("missing updated DB_PORT")
	}
	if !strings.Contains(content, "REDIS_PORT=6379") {
		t.Error("lost unrelated REDIS_PORT")
	}
}

func TestMerge_PreservesBlankLines(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	existing := "FIRST=1\n\nSECOND=2\n"
	os.WriteFile(envPath, []byte(existing), 0644)

	ports := map[string]string{"FIRST": "10"}
	if err := Merge(envPath, ports); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(envPath)
	content := string(data)

	if !strings.Contains(content, "\n\n") {
		t.Error("blank line was removed")
	}
}

func TestMerge_HandlesExportPrefix(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	existing := "export PORT=4000\n"
	os.WriteFile(envPath, []byte(existing), 0644)

	ports := map[string]string{"PORT": "31653"}
	if err := Merge(envPath, ports); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(envPath)
	content := string(data)

	if strings.Contains(content, "4000") {
		t.Error("old value should be overwritten")
	}
	if !strings.Contains(content, "PORT=31653 # managed by outport") {
		t.Error("missing updated PORT")
	}
}

func TestMerge_HandlesQuotedValues(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	existing := "SECRET=\"my secret\"\nPORT=4000\n"
	os.WriteFile(envPath, []byte(existing), 0644)

	ports := map[string]string{"PORT": "31653"}
	if err := Merge(envPath, ports); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(envPath)
	content := string(data)

	if !strings.Contains(content, "SECRET=\"my secret\"") {
		t.Error("lost quoted SECRET value")
	}
	if !strings.Contains(content, "PORT=31653 # managed by outport") {
		t.Error("missing updated PORT")
	}
}

func TestMerge_IsIdempotent(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	ports := map[string]string{"PORT": "31653", "DB_PORT": "17842"}

	Merge(envPath, ports)
	data1, _ := os.ReadFile(envPath)

	Merge(envPath, ports)
	data2, _ := os.ReadFile(envPath)

	if string(data1) != string(data2) {
		t.Errorf("merge is not idempotent:\nfirst:\n%s\nsecond:\n%s", data1, data2)
	}
}

func TestMerge_CommentedOutVarIsNotOverwritten(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	existing := "# PORT=4000\n"
	os.WriteFile(envPath, []byte(existing), 0644)

	ports := map[string]string{"PORT": "31653"}
	if err := Merge(envPath, ports); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(envPath)
	content := string(data)

	if !strings.Contains(content, "# PORT=4000") {
		t.Error("commented line should be preserved")
	}
	if !strings.Contains(content, "PORT=31653 # managed by outport") {
		t.Error("missing appended PORT")
	}
}
