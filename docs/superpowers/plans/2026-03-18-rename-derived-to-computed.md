# Rename "derived" to "computed" Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rename the term "derived" to "computed" throughout the entire Outport codebase — Go source, tests, CLI flags, YAML config key, docs, and skills.

**Architecture:** Pure terminology rename with no logic changes. The YAML config key changes from `derived:` to `computed:`. No backward compatibility shim. Every occurrence of "derived" (as the feature name, not English usage) becomes "computed".

**Tech Stack:** Go, Cobra CLI, YAML, VitePress docs

**Spec:** `docs/superpowers/specs/2026-03-18-rename-derived-to-computed-design.md`

---

### Task 1: Rename types, fields, and functions in `internal/config/config.go`

This is the core package — everything else depends on these exported names.

**Files:**
- Modify: `internal/config/config.go`

- [ ] **Step 1: Rename types**

Replace the following type names:
- `DerivedValue` → `ComputedValue`
- `derivedEnvFileEntry` → `computedEnvFileEntry`
- `derivedEnvFileField` → `computedEnvFileField`
- `rawDerivedValue` → `rawComputedValue`

- [ ] **Step 2: Rename struct fields and YAML/JSON tags**

In `rawConfig`:
- `RawDerived map[string]rawDerivedValue \`yaml:"derived"\`` → `RawComputed map[string]rawComputedValue \`yaml:"computed"\``

In `Config`:
- `Derived map[string]DerivedValue` → `Computed map[string]ComputedValue`

- [ ] **Step 3: Rename exported function**

- `ResolveDerived` → `ResolveComputed`

- [ ] **Step 4: Rename `validateTemplateRefs` parameter and error messages**

- Parameter `derivedName` → `computedName`
- Error messages: `"derived %q: ..."` → `"computed %q: ..."`

- [ ] **Step 5: Update `normalize` method**

- `raw.RawDerived` → `raw.RawComputed`
- `c.Derived[name] = dv` → `c.Computed[name] = dv`

- [ ] **Step 6: Update `Load` function**

- `Derived: make(map[string]DerivedValue)` → `Computed: make(map[string]ComputedValue)`

- [ ] **Step 7: Update `validate` method**

- `c.Derived` → `c.Computed`
- Error messages: `"Derived value %q..."` → `"Computed value %q..."`

- [ ] **Step 8: Update comments**

- Line 16 comment: "in derived value templates" → "in computed value templates"
- Line 84-86 comment: "ResolveDerived substitutes..." → "ResolveComputed substitutes..."
- Line 149 comment: "a derived value's env_file" → "a computed value's env_file"
- Line 156 comment: same pattern

- [ ] **Step 9: Verify it compiles**

Run: `go build ./internal/config/`
Expected: compilation errors in dependent packages (cmd/) — that's expected at this stage.

- [ ] **Step 10: Commit**

```bash
git add internal/config/config.go
git commit -m "refactor: rename derived to computed in config package"
```

---

### Task 2: Update `internal/config/config_test.go`

**Files:**
- Modify: `internal/config/config_test.go`

- [ ] **Step 1: Replace YAML fixture key**

All test YAML strings: `derived:` → `computed:`

These appear in tests:
- `TestLoad_WithDerivedValues` (line 260)
- `TestLoad_DerivedEnvFileArray` (line 295)
- `TestLoad_DerivedPerFileValues` (line 318)
- `TestLoad_DerivedMixedEnvFileEntries` (line 352)
- `TestLoad_DerivedPerFileValidatesReferences` (line 386)
- `TestLoad_DerivedPerFileMissingValue` (line 408)
- `TestLoad_DerivedInvalidReference` (line 454)
- `TestLoad_DerivedInvalidField` (line 474)
- `TestLoad_DerivedNameCollidesWithServiceEnvVar` (line 494)
- `TestLoad_DerivedMissingValue` (line 514)
- `TestLoad_DerivedMissingEnvFile` (line 527)
- `TestTemplateModifierParsing` (line 787)
- `TestTemplateModifierValidation` (line 822)
- `TestURLFieldValidation` (line 846)

- [ ] **Step 2: Replace struct field references**

- `cfg.Derived` → `cfg.Computed` (all occurrences)

- [ ] **Step 3: Replace function calls**

- `ResolveDerived(` → `ResolveComputed(`

- [ ] **Step 4: Rename test functions and comments**

Rename test functions from `*Derived*` to `*Computed*`:
- `TestLoad_WithDerivedValues` → `TestLoad_WithComputedValues`
- `TestLoad_DerivedEnvFileArray` → `TestLoad_ComputedEnvFileArray`
- `TestLoad_DerivedPerFileValues` → `TestLoad_ComputedPerFileValues`
- `TestLoad_DerivedMixedEnvFileEntries` → `TestLoad_ComputedMixedEnvFileEntries`
- `TestLoad_DerivedPerFileValidatesReferences` → `TestLoad_ComputedPerFileValidatesReferences`
- `TestLoad_DerivedPerFileMissingValue` → `TestLoad_ComputedPerFileMissingValue`
- `TestLoad_DerivedInvalidReference` → `TestLoad_ComputedInvalidReference`
- `TestLoad_DerivedInvalidField` → `TestLoad_ComputedInvalidField`
- `TestLoad_DerivedNameCollidesWithServiceEnvVar` → `TestLoad_ComputedNameCollidesWithServiceEnvVar`
- `TestLoad_DerivedMissingValue` → `TestLoad_ComputedMissingValue`
- `TestLoad_DerivedMissingEnvFile` → `TestLoad_ComputedMissingEnvFile`
- `TestResolveDerived_*` → `TestResolveComputed_*` (all of them)
- `TestLoad_NoDerivedIsValid` → `TestLoad_NoComputedIsValid`

Update section comment: `// --- Derived Values ---` → `// --- Computed Values ---`

- [ ] **Step 5: Run config tests**

Run: `go test ./internal/config/ -v`
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add internal/config/config_test.go
git commit -m "test: rename derived to computed in config tests"
```

---

### Task 3: Update `cmd/up.go`

**Files:**
- Modify: `cmd/up.go`

- [ ] **Step 1: Rename types and functions**

- `derivedJSON` → `computedJSON`
- `resolveDerivedFromAlloc` → `resolveComputedFromAlloc`
- `buildDerivedMap` → `buildComputedMap`
- `printDerivedValues` → `printComputedValues`

- [ ] **Step 2: Rename struct fields and JSON tags**

In `upJSON`:
- `Derived map[string]derivedJSON \`json:"derived,omitempty"\`` → `Computed map[string]computedJSON \`json:"computed,omitempty"\``

- [ ] **Step 3: Rename local variables and parameters**

- `resolvedDerived` → `resolvedComputed` (in `runUp`, `printUpJSON`, `printUpStyled`, `mergedEnvFileList`)

- [ ] **Step 4: Update config field references**

- `cfg.Derived` → `cfg.Computed` (in `resolveDerivedFromAlloc` → `resolveComputedFromAlloc`, `buildDerivedMap` → `buildComputedMap`, `printUpJSON`)

- [ ] **Step 5: Update CLI output string**

In `printDerivedValues` (now `printComputedValues`):
- `ui.DimStyle.Render("    derived:")` → `ui.DimStyle.Render("    computed:")`

- [ ] **Step 6: Update comments**

- Line 263: "resolveDerivedFromAlloc resolves derived value templates" → "resolveComputedFromAlloc resolves computed value templates"
- Line 145: `resolvedDerived` param name in comment

- [ ] **Step 7: Commit**

```bash
git add cmd/up.go
git commit -m "refactor: rename derived to computed in up command"
```

---

### Task 4: Update `cmd/ports.go`

**Files:**
- Modify: `cmd/ports.go`

- [ ] **Step 1: Rename flag variable and flag name**

- `portsDerivedFlag` → `portsComputedFlag`
- Flag registration: `"derived"` → `"computed"`, description: `"show derived values"` → `"show computed values"`

- [ ] **Step 2: Update references in `printPortsJSON` and `printPortsStyled`**

- `portsDerivedFlag` → `portsComputedFlag`
- `buildDerivedMap` → `buildComputedMap`
- `resolveDerivedFromAlloc` → `resolveComputedFromAlloc`
- `printDerivedValues` → `printComputedValues`
- `out.Derived` → `out.Computed`
- `cfg.Derived` → `cfg.Computed`

- [ ] **Step 3: Commit**

```bash
git add cmd/ports.go
git commit -m "refactor: rename --derived flag to --computed in ports command"
```

---

### Task 5: Update `cmd/status.go`

**Files:**
- Modify: `cmd/status.go`

- [ ] **Step 1: Rename flag variable and flag name**

- `statusDerivedFlag` → `statusComputedFlag`
- Flag registration: `"derived"` → `"computed"`, description update

- [ ] **Step 2: Rename struct field**

In `statusEntryJSON`:
- `Derived map[string]derivedJSON \`json:"derived,omitempty"\`` → `Computed map[string]computedJSON \`json:"computed,omitempty"\``

- [ ] **Step 3: Update all references**

- `statusDerivedFlag` → `statusComputedFlag`
- `buildDerivedMap` → `buildComputedMap`
- `resolveDerivedFromAlloc` → `resolveComputedFromAlloc`
- `printDerivedValues` → `printComputedValues`
- `cfg.Derived` → `cfg.Computed`
- local var `derived` → `computed`

- [ ] **Step 4: Commit**

```bash
git add cmd/status.go
git commit -m "refactor: rename --derived flag to --computed in status command"
```

---

### Task 6: Update `cmd/share.go`

**Files:**
- Modify: `cmd/share.go`

- [ ] **Step 1: Rename struct field**

In `shareJSON`:
- `Derived map[string]derivedJSON \`json:"derived,omitempty"\`` → `Computed map[string]computedJSON \`json:"computed,omitempty"\``

- [ ] **Step 2: Update references**

- `resolvedDerived` → `resolvedComputed`
- `buildDerivedMap` → `buildComputedMap`
- `cfg.Derived` → `cfg.Computed`
- `out.Derived` → `out.Computed`

- [ ] **Step 3: Commit**

```bash
git add cmd/share.go
git commit -m "refactor: rename derived to computed in share command"
```

---

### Task 7: Update `cmd/down.go` and `cmd/rename.go`

**Files:**
- Modify: `cmd/down.go`
- Modify: `cmd/rename.go`

- [ ] **Step 1: Update `cmd/down.go`**

In `cleanEnvFiles`:
- `cfg.Derived` → `cfg.Computed`

- [ ] **Step 2: Update `cmd/rename.go`**

- `resolveDerivedFromAlloc` → `resolveComputedFromAlloc`
- `resolvedDerived` → `resolvedComputed`
- Comments: "resolved derived values" → "resolved computed values", "Resolve derived values" → "Resolve computed values"

- [ ] **Step 3: Commit**

```bash
git add cmd/down.go cmd/rename.go
git commit -m "refactor: rename derived to computed in down and rename commands"
```

---

### Task 8: Update `cmd/init.go`

**Files:**
- Modify: `cmd/init.go`

- [ ] **Step 1: Update template comments**

In `configTemplate`:
- `# Derived values — computed env vars that reference allocated ports:` → `# Computed values — env vars that reference allocated ports:`
- `# derived:` → `# computed:`

- [ ] **Step 2: Commit**

```bash
git add cmd/init.go
git commit -m "refactor: rename derived to computed in init template"
```

---

### Task 9: Update `cmd/cmd_test.go`

Must be done before running tests — the test file references renamed types/functions from Tasks 1-8 and won't compile otherwise.

**Files:**
- Modify: `cmd/cmd_test.go`

- [ ] **Step 1: Replace YAML fixture key**

All test YAML strings containing `derived:` → `computed:`

This includes:
- `testConfigWithDerived` constant (line 212)
- `TestUp_ComputedPerFileValues` inline YAML (line 300)
- `testConfigWithMultipleHostnames` inline YAML (line 1142)
- Share test inline YAML (line 1365)
- `testConfigWithDerivedAndHostnames` constant (line 1726)

- [ ] **Step 2: Replace struct field references**

- `result.Derived` → `result.Computed`
- `cfg.Derived` → `cfg.Computed` (if any)

- [ ] **Step 3: Replace function/type references**

- `derivedJSON` → `computedJSON`
- `buildDerivedMap` → `buildComputedMap`
- `resolveDerivedFromAlloc` → `resolveComputedFromAlloc`
- `printDerivedValues` → `printComputedValues`
- `config.DerivedValue` → `config.ComputedValue`

- [ ] **Step 4: Rename test functions and constants**

- `testConfigWithDerived` → `testConfigWithComputed`
- `testConfigWithDerivedAndHostnames` → `testConfigWithComputedAndHostnames`
- `TestUp_WithDerivedValues` → `TestUp_WithComputedValues`
- `TestUp_DerivedStyledOutput` → `TestUp_ComputedStyledOutput`
- `TestUp_DerivedPerFileValues` → `TestUp_ComputedPerFileValues`
- `TestUp_NoDerived_OmitsFromJSON` → `TestUp_NoComputed_OmitsFromJSON`
- `TestPrintShareJSON_IncludesDerivedValues` → `TestPrintShareJSON_IncludesComputedValues`

- [ ] **Step 5: Update string assertions**

- `"derived:"` → `"computed:"` in styled output assertions
- `"derived"` → `"computed"` in JSON omission checks
- Comments: "derived values" → "computed values"

- [ ] **Step 6: Commit**

```bash
git add cmd/cmd_test.go
git commit -m "test: rename derived to computed in cmd tests"
```

---

### Task 10: Verify all Go code compiles and passes tests

- [ ] **Step 1: Build**

Run: `just build`
Expected: compiles cleanly

- [ ] **Step 2: Lint**

Run: `just lint`
Expected: no errors

- [ ] **Step 3: Run all tests**

Run: `just test`
Expected: all PASS

---

### Task 11: Update documentation

**Files:**
- Modify: `docs/reference/configuration.md`
- Modify: `docs/reference/commands.md`
- Modify: `docs/guide/getting-started.md`
- Modify: `docs/guide/examples.md`
- Modify: `docs/guide/tips.md`
- Modify: `docs/.vitepress/theme/HomeLayout.vue`
- Modify: `docs/guide/work-with-ai.md`
- Modify: `README.md`
- Modify: `CLAUDE.md`
- Modify: `skills/outport/SKILL.md`

- [ ] **Step 1: Update `docs/reference/configuration.md`**

- Section heading `### \`derived\`` → `### \`computed\``
- All YAML examples: `derived:` → `computed:`
- All prose: "derived values" → "computed values", "Derived values" → "Computed values"

- [ ] **Step 2: Update `docs/reference/commands.md`**

- `--derived` → `--computed`

- [ ] **Step 3: Update `docs/guide/getting-started.md`**

- "derived values" → "computed values"

- [ ] **Step 4: Update `docs/guide/examples.md`**

- YAML examples: `derived:` → `computed:`
- Prose: "derived values" → "computed values"

- [ ] **Step 5: Update `docs/guide/tips.md`**

- "derived values" → "computed values"

- [ ] **Step 6: Update `docs/.vitepress/theme/HomeLayout.vue`**

- "derived values" → "computed values"

- [ ] **Step 7: Update `docs/guide/work-with-ai.md`**

- "Derived values" → "Computed values"

- [ ] **Step 8: Update `README.md`**

- Section heading: "Derived Values" → "Computed Values"
- YAML examples: `derived:` → `computed:`
- `--derived` → `--computed`

- [ ] **Step 9: Update `CLAUDE.md`**

- All references to "derived" in the architecture description → "computed"
- `DerivedValue` → `ComputedValue`, `ResolveDerived` → `ResolveComputed`
- `cfg.Derived` → `cfg.Computed`
- `derived:` YAML key → `computed:`
- `--derived` flag → `--computed`
- Prose: "derived values" → "computed values"

- [ ] **Step 10: Update `skills/outport/SKILL.md`**

- Section heading, YAML examples, `--derived` → `--computed`, prose

- [ ] **Step 11: Verify docs build**

Run: `npm run docs:build --prefix docs` (or equivalent)
Expected: builds cleanly

- [ ] **Step 12: Commit**

```bash
git add docs/ README.md CLAUDE.md skills/
git commit -m "docs: rename derived to computed across all documentation"
```

---

### Task 12: Rename and update spec file

**Files:**
- Rename: `project/specs/derived-values.md` → `project/specs/computed-values.md`
- Modify: `project/specs/index.md`
- Modify: `project/research.md`

- [ ] **Step 1: Rename spec file**

```bash
git mv project/specs/derived-values.md project/specs/computed-values.md
```

- [ ] **Step 2: Update content of renamed file**

Replace all "derived" → "computed" in `project/specs/computed-values.md`:
- `DerivedValue` → `ComputedValue`
- `ResolveDerived` → `ResolveComputed`
- `derived:` YAML key → `computed:`
- All prose

- [ ] **Step 3: Update `project/specs/index.md`**

- Link path and text: "derived-values" → "computed-values", "Derived Values" → "Computed Values"

- [ ] **Step 4: Update `project/research.md`**

- "derived values" → "computed values"

- [ ] **Step 5: Commit**

```bash
git add project/
git commit -m "docs: rename derived-values spec to computed-values"
```

---

### Task 13: Update superpowers docs

**Files:**
- Modify: `docs/superpowers/specs/2026-03-18-tunnel-url-orchestration-design.md`
- Modify: `docs/superpowers/plans/2026-03-18-tunnel-url-orchestration.md`
- Modify: `docs/superpowers/specs/2026-03-17-cli-command-restructure-design.md`
- Modify: `docs/superpowers/plans/2026-03-17-cli-command-restructure.md`
- Modify: `project/superpowers/plans/2026-03-13-outport-v1.md`
- Modify: `project/superpowers/plans/2026-03-15-test-domains.md`
- Modify: `project/superpowers/plans/2026-03-16-docs-site.md`
- Modify: `project/superpowers/specs/2026-03-15-test-domains-design.md`
- Modify: `project/superpowers/specs/2026-03-16-docs-site-design.md`
- Modify: `docs/superpowers/plans/2026-03-16-local-ssl.md`

**Note:** `docs/superpowers/specs/2026-03-17-outport-share-design.md` uses "derived" in the English sense — leave it as-is.

- [ ] **Step 1: Update all files**

In each file, replace feature-name usage of "derived":
- `derived:` YAML key → `computed:`
- `ResolveDerived` → `ResolveComputed`
- `resolveDerivedFromAlloc` → `resolveComputedFromAlloc`
- `--derived` → `--computed`
- `cfg.Derived` → `cfg.Computed`
- "derived values" → "computed values" (when referring to the feature, not English usage)

- [ ] **Step 2: Commit**

```bash
git add docs/superpowers/ project/superpowers/
git commit -m "docs: rename derived to computed in superpowers docs"
```

---

### Task 14: Final verification

- [ ] **Step 1: Build and test**

Run: `just build && just lint && just test`
Expected: all pass

- [ ] **Step 2: Grep for remaining "derived" references**

Run: `grep -ri "derived" --include="*.go" --include="*.md" --include="*.vue" --include="*.yml" .`

Review output — any remaining hits should be either:
- English usage ("derived from", "derived context") — leave as-is
- This plan/spec document itself — leave as-is
- False positives — leave as-is

- [ ] **Step 3: Test JSON output**

Run `outport up --json` in a project directory and verify the key is `"computed"` not `"derived"`.

- [ ] **Step 4: Test CLI flags**

Run `outport ports --computed` and `outport system status --computed` to verify the new flag name works.
