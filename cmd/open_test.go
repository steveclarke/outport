package cmd

import (
	"strings"
	"testing"
)

const testConfigWithAliases = `name: testapp
services:
  web:
    preferred_port: 3000
    env_var: PORT
    hostname: testapp.test
    aliases:
      admin: admin.testapp.test
      worship: worship.testapp.test
  worship:
    preferred_port: 3001
    env_var: WORSHIP_PORT
    hostname: service-worship.testapp.test
`

const testConfigWithDuplicateAliasNames = `name: testapp
services:
  web:
    preferred_port: 3000
    env_var: PORT
    hostname: testapp.test
    aliases:
      admin: admin.testapp.test
  api:
    preferred_port: 3001
    env_var: API_PORT
    hostname: api.testapp.test
    aliases:
      admin: api-admin.testapp.test
`

func captureOpenBrowser(t *testing.T) *[]string {
	t.Helper()

	var opened []string
	old := openBrowserFunc
	openBrowserFunc = func(url string) error {
		opened = append(opened, url)
		return nil
	}
	t.Cleanup(func() {
		openBrowserFunc = old
	})

	return &opened
}

// openResult mirrors the JSON envelope emitted by printOpenJSON.
type openResult struct {
	Opened []openTarget `json:"opened"`
}

func TestOpen_ServiceNameWinsOverAlias(t *testing.T) {
	setupProject(t, testConfigWithAliases)
	executeCmd(t, "up")
	opened := captureOpenBrowser(t)

	output := executeCmd(t, "open", "worship", "--json")

	if len(*opened) != 1 {
		t.Fatalf("opened %d URLs, want 1", len(*opened))
	}
	if !strings.HasSuffix((*opened)[0], "://service-worship.testapp.test") {
		t.Fatalf("opened URL = %q, want service hostname", (*opened)[0])
	}

	var result openResult
	unwrapJSON(t, output, &result)

	if len(result.Opened) != 1 {
		t.Fatalf("opened entries = %d, want 1", len(result.Opened))
	}
	got := result.Opened[0]
	if got.Kind != "service" || got.Service != "worship" || got.Alias != "" || got.Hostname != "service-worship.testapp.test" || got.Port != 3001 {
		t.Fatalf("opened entry = %+v, want service worship", got)
	}
}

func TestOpen_UniqueAliasName(t *testing.T) {
	setupProject(t, testConfigWithAliases)
	executeCmd(t, "up")
	opened := captureOpenBrowser(t)

	output := executeCmd(t, "open", "admin", "--json")

	if len(*opened) != 1 {
		t.Fatalf("opened %d URLs, want 1", len(*opened))
	}
	if !strings.HasSuffix((*opened)[0], "://admin.testapp.test") {
		t.Fatalf("opened URL = %q, want alias hostname", (*opened)[0])
	}

	var result openResult
	unwrapJSON(t, output, &result)

	if len(result.Opened) != 1 {
		t.Fatalf("opened entries = %d, want 1", len(result.Opened))
	}
	got := result.Opened[0]
	if got.Kind != "alias" || got.Service != "web" || got.Alias != "admin" || got.Hostname != "admin.testapp.test" || got.Port != 3000 {
		t.Fatalf("opened entry = %+v, want web admin alias", got)
	}
}

func TestOpen_ExplicitAliasTarget(t *testing.T) {
	setupProject(t, testConfigWithAliases)
	executeCmd(t, "up")
	opened := captureOpenBrowser(t)

	output := executeCmd(t, "open", "web:worship", "--json")

	if len(*opened) != 1 {
		t.Fatalf("opened %d URLs, want 1", len(*opened))
	}
	if !strings.HasSuffix((*opened)[0], "://worship.testapp.test") {
		t.Fatalf("opened URL = %q, want explicit alias hostname", (*opened)[0])
	}

	var result openResult
	unwrapJSON(t, output, &result)

	if len(result.Opened) != 1 {
		t.Fatalf("opened entries = %d, want 1", len(result.Opened))
	}
	got := result.Opened[0]
	if got.Kind != "alias" || got.Service != "web" || got.Alias != "worship" || got.Hostname != "worship.testapp.test" || got.Port != 3000 {
		t.Fatalf("opened entry = %+v, want web worship alias", got)
	}
}

func TestOpen_AmbiguousAliasNameErrors(t *testing.T) {
	setupProject(t, testConfigWithDuplicateAliasNames)
	executeCmd(t, "up")
	captureOpenBrowser(t)

	_, err := executeCmdAllowError(t, "open", "admin")

	if err == nil {
		t.Fatal("expected ambiguous alias error")
	}
	if !strings.Contains(err.Error(), `alias "admin" is ambiguous`) {
		t.Fatalf("error = %v, want ambiguous alias message", err)
	}
	if !strings.Contains(err.Error(), "web:admin") || !strings.Contains(err.Error(), "api:admin") {
		t.Fatalf("error = %v, want explicit target suggestions", err)
	}
}
