# Rename "derived" to "computed"

**Date:** 2026-03-18
**Status:** Approved

## Summary

Rename the term "derived" to "computed" throughout the Outport codebase. This is a terminology change only — no logic, behavior, or architecture changes. The YAML config key changes from `derived:` to `computed:`. No backward compatibility shim; clean break.

## Motivation

"Derived" is technically accurate but clunky. "Computed" is shorter, easier to say, and more familiar to developers (Vue `computed`, CSS `calc()`, spreadsheet formulas). It better emphasizes the result — "here's a value we figured out" — which is what users care about.

## Scope

### Go types and structs

| Current | New |
|---------|-----|
| `DerivedValue` | `ComputedValue` |
| `rawDerivedValue` | `rawComputedValue` |
| `derivedEnvFileEntry` | `computedEnvFileEntry` |
| `derivedEnvFileField` | `computedEnvFileField` |
| `derivedJSON` | `computedJSON` |

### Go struct fields

| Current | New |
|---------|-----|
| `Config.Derived` | `Config.Computed` |
| `Config.RawDerived` (YAML tag `derived`) | `Config.RawComputed` (YAML tag `computed`) |
| `upJSON.Derived` (JSON tag `derived`) | `upJSON.Computed` (JSON tag `computed`) |
| `statusEntryJSON.Derived` | `statusEntryJSON.Computed` |
| `shareJSON.Derived` | `shareJSON.Computed` |

### Go functions

| Current | New |
|---------|-----|
| `ResolveDerived()` | `ResolveComputed()` |
| `resolveDerivedFromAlloc()` | `resolveComputedFromAlloc()` |
| `buildDerivedMap()` | `buildComputedMap()` |
| `printDerivedValues()` | `printComputedValues()` |
| `validateTemplateRefs()` param `derivedName` | `computedName` |
| `mergedEnvFileList()` param `resolvedDerived` | `resolvedComputed` |

### Go variables

| Current | New |
|---------|-----|
| `portsDerivedFlag` | `portsComputedFlag` |
| `statusDerivedFlag` | `statusComputedFlag` |
| Local vars named `derived`, `resolvedDerived` | `computed`, `resolvedComputed` |

### Additional Go files

- `cmd/down.go` — references `cfg.Derived`, must become `cfg.Computed`
- `cmd/rename.go` — comments referencing "derived values" and calls to `resolveDerivedFromAlloc()`
- `cmd/ports.go` — constructs `upJSON` with `Derived` field beyond just the flag variable

### CLI flags

| Current | New |
|---------|-----|
| `--derived` on `ports` command | `--computed` |
| `--derived` on `status` command | `--computed` |

### CLI output

| Current | New |
|---------|-----|
| `"derived:"` section label in `up` output | `"computed:"` |

### YAML config key

| Current | New |
|---------|-----|
| `derived:` in `.outport.yml` | `computed:` |

No fallback or migration. Old configs using `derived:` will get an error from YAML unmarshaling (unknown key). Users update their `.outport.yml` and install the new binary.

### Error messages and comments

All error messages like `"Derived value %q..."` become `"Computed value %q..."`. All code comments updated to match.

### Init template

The `cmd/init.go` preset comment and example YAML updated from `derived:` to `computed:`.

### Tests

All test YAML fixtures, variable names, assertions, and comments in `cmd/cmd_test.go` and `internal/config/config_test.go` updated.

### Documentation

| File | Changes |
|------|---------|
| `docs/reference/configuration.md` | Section heading, YAML examples, prose |
| `docs/reference/commands.md` | `--computed` flag references |
| `docs/guide/getting-started.md` | Prose references |
| `docs/guide/examples.md` | YAML examples, prose |
| `docs/guide/tips.md` | Prose references |
| `docs/.vitepress/theme/HomeLayout.vue` | "derived values" in homepage description |
| `README.md` | "Derived Values" section heading, YAML examples, `--derived` flag reference |
| `CLAUDE.md` | Architecture description |
| `skills/outport/SKILL.md` | Section heading, YAML examples, `--derived` flag, prose |

### Spec file rename

`project/specs/derived-values.md` → `project/specs/computed-values.md` (content updated too).

### Project docs

| File | Changes |
|------|---------|
| `project/specs/index.md` | Link text and path to renamed spec file |
| `project/research.md` | Prose references to "derived values" |

### Superpowers docs

| File | Changes |
|------|---------|
| `docs/superpowers/specs/2026-03-18-tunnel-url-orchestration-design.md` | Template vars, `ResolveDerived`, prose |
| `docs/superpowers/plans/2026-03-18-tunnel-url-orchestration.md` | `resolveDerivedFromAlloc`, `derived` config key, prose |
| `docs/superpowers/specs/2026-03-17-cli-command-restructure-design.md` | `--derived` flag references |
| `docs/superpowers/plans/2026-03-17-cli-command-restructure.md` | `--derived` flag reference |
| `project/superpowers/plans/2026-03-13-outport-v1.md` | Prose references |
| `project/superpowers/plans/2026-03-15-test-domains.md` | Prose references |
| `project/superpowers/plans/2026-03-16-docs-site.md` | Prose references |
| `project/superpowers/specs/2026-03-15-test-domains-design.md` | Prose references |
| `project/superpowers/specs/2026-03-16-docs-site-design.md` | Prose references |

Note: `docs/superpowers/specs/2026-03-17-outport-share-design.md` uses "derived" in the English sense (not the feature name) — leave as-is.

## What does NOT change

- Port allocation logic
- Template expansion syntax (`${service.port}`, `${var:-default}`, etc.)
- `.env` file fencing behavior
- Registry format
- Any runtime behavior

## Testing

Run `just test` and `just lint` after the rename. All tests should pass with no logic changes — only identifier and string updates.
