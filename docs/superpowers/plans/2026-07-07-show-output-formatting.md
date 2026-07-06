# Show Output Formatting Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `taskgate show` list summaries in an aligned column, and add a `run`-style `name` field to every `taskgate ai show` record/envelope.

**Architecture:** Two independent rendering changes in `internal/show`. Human tree rendering switches from a single tab separator to a two-pass, globally-aligned summary column. AI rendering gains one additive field (`name`) derived from each entry's physical path by stripping its `.taskgate/<bucket>/` prefix. No change to resolution, collision detection, audience merging, recursion depth, or exit codes.

**Tech Stack:** Go (standard library only: `fmt`, `io`, `strings`, `unicode/utf8`, `encoding/json`). Unit tests are plain `testing`; e2e tests are Ginkgo/Gomega with golden files.

## Global Constraints

- Spec: `docs/superpowers/specs/2026-07-07-show-output-formatting-design.md`.
- Go module root is the repo root; the `show` package lives at `taskgate/internal/show/`. Run Go commands from the repo root.
- AI wire format is **additive only** (ADR-0003): add `name`, never remove/rename existing fields. `summary` stays `null` (not omitted) when absent; `body` stays `omitempty`.
- `name` = the entry's `Path` with the leading `.taskgate/<bucket>/` removed (`bucket` ∈ `{human, ai, shared}`). Examples: `.taskgate/human/build` → `build`; `.taskgate/shared/deploy/prod` → `deploy/prod`; `.taskgate/shared/deploy` → `deploy`.
- Human summary column: one column across the whole listing; start = `max(2*depth + runeCount(name))` over task rows carrying a non-empty (trimmed) summary, plus a **two-space** gap. Directory rows and summary-less task rows print name-only, no trailing padding. No truncation.
- Commit after each task. Conventional Commits (`feat`/`fix`/`test`/`docs`, imperative subject, ≤72 chars).

---

### Task 1: `run`-style name helper

**Files:**
- Modify: `taskgate/internal/show/render_ai.go`
- Test: `taskgate/internal/show/render_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `func runName(path string) string` — strips the `.taskgate/<bucket>/` prefix from a physical path and returns the slash-joined remainder. Returns `""` for a path with fewer than three segments.

- [ ] **Step 1: Write the failing test**

Add to `taskgate/internal/show/render_test.go`:

```go
func TestRunName(t *testing.T) {
	cases := map[string]string{
		".taskgate/human/build":        "build",
		".taskgate/shared/deploy/prod":  "deploy/prod",
		".taskgate/shared/deploy":       "deploy",
		".taskgate/ai/deep/nested/task": "deep/nested/task",
	}
	for path, want := range cases {
		if got := runName(path); got != want {
			t.Errorf("runName(%q) = %q, want %q", path, got, want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./taskgate/internal/show/ -run TestRunName -v`
Expected: FAIL — `undefined: runName`.

- [ ] **Step 3: Write minimal implementation**

Add to `taskgate/internal/show/render_ai.go` (the `strings` import is new for this file):

```go
import (
	"encoding/json"
	"io"
	"strings"
)

// runName maps a physical entry path onto its run-style name by dropping the
// ".taskgate/<bucket>/" prefix (bucket is human, ai, or shared). Returns ""
// when path has fewer than three slash-separated segments.
func runName(path string) string {
	segs := strings.Split(path, "/")
	if len(segs) < 3 {
		return ""
	}
	return strings.Join(segs[2:], "/")
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./taskgate/internal/show/ -run TestRunName -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add taskgate/internal/show/render_ai.go taskgate/internal/show/render_test.go
git commit -m "feat(show): add runName helper for ai wire format"
```

---

### Task 2: Add `name` to the AI wire format

**Files:**
- Modify: `taskgate/internal/show/render_ai.go`
- Modify: `taskgate/internal/show/show.go:123-144` (`renderAITarget`)
- Test: `taskgate/internal/show/render_test.go`

**Interfaces:**
- Consumes: `runName(path string) string` (Task 1).
- Produces: `childRecord`, `taskEnvelope`, and `directoryEnvelope` each carry a `Name string json:"name"` field. `childRecords` and `renderAITarget` populate it via `runName`.

- [ ] **Step 1: Write the failing tests**

In `taskgate/internal/show/render_test.go`, extend the existing assertions.

In `TestRenderAI_Listing`, after the `r0` block, add:

```go
	if r0["name"] != "deploy" {
		t.Errorf("row0 name = %v, want deploy", r0["name"])
	}
	if r1["name"] != "lint" {
		t.Errorf("row1 name = %v, want lint", r1["name"])
	}
```

In `TestRenderAI_Task`, after the `kind` check, add:

```go
	if got["name"] != "lint" {
		t.Errorf("name = %v, want lint", got["name"])
	}
```

In `TestRenderAI_Directory`, after the `kind` check, add:

```go
	if got["name"] != "deploy" {
		t.Errorf("name = %v, want deploy", got["name"])
	}
	r0 := rows[0].(map[string]any)
	if r0["name"] != "deploy/prod" {
		t.Errorf("child name = %v, want deploy/prod", r0["name"])
	}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./taskgate/internal/show/ -run 'TestRenderAI_(Listing|Task|Directory)' -v`
Expected: FAIL — `name` keys are absent (`want deploy`, got `<nil>`).

- [ ] **Step 3: Add the `Name` fields and populate them**

In `taskgate/internal/show/render_ai.go`, add `Name` to the three types (place `name` first so it reads as the primary identifier):

```go
type childRecord struct {
	Name    string  `json:"name"`
	Path    string  `json:"path"`
	Kind    string  `json:"kind"`
	Summary *string `json:"summary"`
}

type taskEnvelope struct {
	Kind     string  `json:"kind"`
	Name     string  `json:"name"`
	Path     string  `json:"path"`
	Summary  *string `json:"summary"`
	Body     string  `json:"body,omitempty"`
	Audience string  `json:"audience"`
}

type directoryEnvelope struct {
	Kind     string        `json:"kind"`
	Name     string        `json:"name"`
	Path     string        `json:"path"`
	Audience string        `json:"audience"`
	Entries  []childRecord `json:"entries"`
}
```

In the same file, populate the child record name in `childRecords`:

```go
func childRecords(entries []Entry) []childRecord {
	out := make([]childRecord, 0, len(entries))
	for _, e := range entries {
		out = append(out, childRecord{
			Name:    runName(e.Path),
			Path:    e.Path,
			Kind:    kindString(e.Kind),
			Summary: summaryPtr(e.Annotation.Summary),
		})
	}
	return out
}
```

In `taskgate/internal/show/show.go`, populate the envelope names in `renderAITarget`:

```go
func renderAITarget(w io.Writer, target ResolvedTarget) error {
	switch target.Kind {
	case EntryKindTask:
		env := taskEnvelope{
			Kind:     "task",
			Name:     runName(target.Entry.Path),
			Path:     target.Entry.Path,
			Summary:  summaryPtr(target.Entry.Annotation.Summary),
			Body:     target.Entry.Annotation.Body,
			Audience: "ai",
		}
		return renderAI(w, env)
	case EntryKindDirectory:
		env := directoryEnvelope{
			Kind:     "directory",
			Name:     runName(target.Entry.Path),
			Path:     target.Entry.Path,
			Audience: "ai",
			Entries:  childRecords(target.Children),
		}
		return renderAI(w, env)
	}
	return fmt.Errorf("unknown target kind")
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./taskgate/internal/show/ -run 'TestRenderAI' -v`
Expected: PASS (all AI render tests, including the unchanged error/summary/body cases).

- [ ] **Step 5: Commit**

```bash
git add taskgate/internal/show/render_ai.go taskgate/internal/show/show.go taskgate/internal/show/render_test.go
git commit -m "feat(show): add run-style name to ai show records and envelopes"
```

---

### Task 3: Aligned summary column in the human tree

**Files:**
- Modify: `taskgate/internal/show/render_human.go`
- Test: `taskgate/internal/show/render_test.go`

**Interfaces:**
- Consumes: `Entry` (fields `Name`, `Kind`, `Depth`, `Annotation.Summary`).
- Produces: internal helpers `taskRowWidth(e Entry, depth int) int` and `summaryColumn(entries []Entry, depth int, useEntryDepth bool) int`; `writeTreeRow` gains a `col int` parameter. `RenderHumanTree` and `RenderHumanDirectory` signatures are unchanged.

- [ ] **Step 1: Update the failing unit tests**

Replace the `want` line in `TestRenderHumanTree_IndentsByDepth`:

```go
	want := "deploy/\n  prod  Prod.\nbuild   Build.\nbare\n"
```

Replace the `want` line in `TestRenderHumanDirectory_PathThenChildren`:

```go
	want := ".taskgate/human/deploy\n\n  sub/\n  canary  Canary.\n  prod\n"
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./taskgate/internal/show/ -run 'TestRenderHuman(Tree|Directory)' -v`
Expected: FAIL — current output still uses `\t` (e.g. got `  prod\tProd.`).

- [ ] **Step 3: Rewrite the human tree rendering**

First, add `"unicode/utf8"` to the existing import block at the top of `taskgate/internal/show/render_human.go` so it reads:

```go
import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)
```

Then replace everything from `writeTreeRow` to the end of the file (keep `RenderHumanTask` above it untouched) with:

```go
// taskRowWidth is the printed width of a task row's "indent + name" prefix:
// two spaces per depth level plus the rune count of the basename.
func taskRowWidth(e Entry, depth int) int {
	return 2*depth + utf8.RuneCountInString(e.Name)
}

// hasSummary reports whether a task entry carries a non-empty trimmed summary.
func hasSummary(e Entry) bool {
	return e.Kind == EntryKindTask && strings.TrimSpace(e.Annotation.Summary) != ""
}

// summaryColumn returns the column (character offset) where summaries begin:
// the widest summary-bearing task row plus a two-space gap. When useEntryDepth
// is true each entry's own Depth is used (recursive tree); otherwise the fixed
// depth argument applies to every entry (single-level directory listing).
// Returns 0 when no row carries a summary, signalling "no column".
func summaryColumn(entries []Entry, depth int, useEntryDepth bool) int {
	widest := 0
	for _, e := range entries {
		if !hasSummary(e) {
			continue
		}
		d := depth
		if useEntryDepth {
			d = e.Depth
		}
		if w := taskRowWidth(e, d); w > widest {
			widest = w
		}
	}
	if widest == 0 {
		return 0
	}
	return widest + 2
}

// writeTreeRow writes a single indented tree row: two spaces per depth, then
// the basename. Directories get a trailing "/". Task rows with a summary pad
// the name out to col before printing the trimmed summary; task rows without
// a summary (or when col == 0) print the name alone.
func writeTreeRow(w io.Writer, e Entry, depth, col int) error {
	indent := strings.Repeat("  ", depth)
	if e.Kind == EntryKindDirectory {
		_, err := fmt.Fprintf(w, "%s%s/\n", indent, e.Name)
		return err
	}
	summary := strings.TrimSpace(e.Annotation.Summary)
	if summary == "" || col == 0 {
		_, err := fmt.Fprintf(w, "%s%s\n", indent, e.Name)
		return err
	}
	pad := col - taskRowWidth(e, depth)
	if pad < 1 {
		pad = 1
	}
	_, err := fmt.Fprintf(w, "%s%s%s%s\n", indent, e.Name, strings.Repeat(" ", pad), summary)
	return err
}

// RenderHumanTree writes the recursive listing as an indented tree, one row
// per entry, indented by Entry.Depth, with summaries in a single aligned
// column spanning the whole tree.
func RenderHumanTree(w io.Writer, entries []Entry) error {
	col := summaryColumn(entries, 0, true)
	for _, e := range entries {
		if err := writeTreeRow(w, e, e.Depth, col); err != nil {
			return err
		}
	}
	return nil
}

// RenderHumanDirectory writes the directory-target view: the directory's real
// path, a blank line, then its immediate children as one-level tree rows with
// summaries aligned within this listing. Directories carry no summary/body.
func RenderHumanDirectory(w io.Writer, target ResolvedTarget) error {
	if _, err := fmt.Fprintln(w, target.Entry.Path); err != nil {
		return err
	}
	if len(target.Children) > 0 {
		col := summaryColumn(target.Children, 1, false)
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
		for _, c := range target.Children {
			if err := writeTreeRow(w, c, 1, col); err != nil {
				return err
			}
		}
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./taskgate/internal/show/ -v`
Expected: PASS (all `show` unit tests, including the AI tests from Task 2).

- [ ] **Step 5: Commit**

```bash
git add taskgate/internal/show/render_human.go taskgate/internal/show/render_test.go
git commit -m "feat(show): align summary column in human tree output"
```

---

### Task 4: Update e2e golden files and AI assertions

**Files:**
- Modify: `tests/e2e/show/testdata/golden/browse_recursive.golden`
- Modify: `tests/e2e/show/testdata/golden/dir_children.golden`
- Modify: `tests/e2e/show/browse_test.go`
- Modify: `tests/e2e/show/directory_test.go`

**Interfaces:**
- Consumes: the rendered CLI output from Tasks 2–3.
- Produces: golden files matching the aligned column; e2e AI tests asserting the `name` field.

- [ ] **Step 1: Update the human golden files**

Set `tests/e2e/show/testdata/golden/browse_recursive.golden` to exactly (note the two spaces after `prod`, three after `stg` and `build`, and a trailing newline):

```
deploy/
  prod  Prod.
  stg   Stg.
build   Build.
```

Set `tests/e2e/show/testdata/golden/dir_children.golden` to exactly (two spaces after `canary`, four after `prod`, trailing newline):

```
.taskgate/human/deploy

  canary  Promote to canary.
  prod    Promote to production.
```

- [ ] **Step 2: Add `name` assertions to the AI e2e tests**

In `tests/e2e/show/browse_test.go`, inside the `ai show with no argument` `It` block, after the existing `ContainSubstring` assertions add:

```go
			Expect(out.Stdout).To(ContainSubstring(`"name":"analyze"`))
			Expect(out.Stdout).To(ContainSubstring(`"name":"deep/nested"`))
			Expect(out.Stdout).To(ContainSubstring(`"name":"deep"`))
```

In `tests/e2e/show/directory_test.go`, inside the `directory envelope` `It` block, after the `ContainSubstring` path assertions add:

```go
			Expect(envelope["name"]).To(Equal("deploy"))
			Expect(out.Stdout).To(ContainSubstring(`"name":"deploy/canary"`))
			Expect(out.Stdout).To(ContainSubstring(`"name":"deploy/prod"`))
```

- [ ] **Step 3: Run the e2e suite to verify everything passes**

Run: `go test ./tests/e2e/show/...`
Expected: PASS (goldens match the aligned column; AI envelopes carry `name`).

- [ ] **Step 4: Run the full show test surface**

Run: `go test ./taskgate/internal/show/... ./tests/e2e/show/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add tests/e2e/show/testdata/golden/browse_recursive.golden tests/e2e/show/testdata/golden/dir_children.golden tests/e2e/show/browse_test.go tests/e2e/show/directory_test.go
git commit -m "test(show): update goldens for aligned column and ai name field"
```

---

### Task 5: Update documentation

**Files:**
- Modify: `docs/show/adr/0003-ai-output-wire-format.md`
- Modify: `docs/show/glossary.md`
- Modify: `docs/show/requirements.md`

**Interfaces:**
- Consumes: the implemented wire format and human format.
- Produces: docs consistent with the shipped behavior. No code.

- [ ] **Step 1: Update ADR-0003 wire format**

In `docs/show/adr/0003-ai-output-wire-format.md`, replace the four-shapes JSON block with:

```json
{"kind":"listing",   "audience":"…", "entries":[ {"name":"…","path":"…","kind":"task|directory","summary":"…|null"}, … ]}
{"kind":"task",      "name":"…", "path":"…", "summary":"…|null", "body":"…", "audience":"…"}
{"kind":"directory", "name":"…", "path":"…", "audience":"…", "entries":[ … ]}
{"kind":"error",     "error":"<code>", "message":"…", …}
```

Then, in the "Field rules" list, add one bullet after the `summary` bullet:

```markdown
- `name` is the entry's `run`-style name (bare or slash-separated), present on every entry-describing record and on the `task`/`directory` envelopes. It equals `path` with the `.taskgate/<bucket>/` prefix removed. `path` remains the physical location; `name` is what `taskgate run` / `taskgate ai show` accept.
```

And update the children bullet to read:

```markdown
- Children in `entries[]` carry `name` + `path` + `kind` + `summary` only — never `body`, never recursive `entries`.
```

- [ ] **Step 2: Update the glossary**

In `docs/show/glossary.md`, in the "## Output record" section, append this sentence to the paragraph:

```markdown
In the AI form each record and the file/directory envelopes also carry a `name`: the entry's `run`-style name (its physical `path` minus the `.taskgate/<bucket>/` prefix).
```

- [ ] **Step 3: Update requirements**

In `docs/show/requirements.md`, append to the **FR-006** bullet:

```markdown
 Each AI-form record and the file/directory envelopes additionally carry a `name` field holding the entry's `run`-style name (the physical path minus its `.taskgate/<bucket>/` prefix), so an AI client has the exact string to pass back to `taskgate run` / `taskgate ai show` without reconstructing it from `path`.
```

- [ ] **Step 4: Verify docs reference no removed fields**

Run: `git -C . grep -n '"kind":"directory"' docs/show`
Expected: the directory envelope example no longer lists `summary`/`body` for the directory itself; confirm the ADR block matches Step 1.

- [ ] **Step 5: Commit**

```bash
git add docs/show/adr/0003-ai-output-wire-format.md docs/show/glossary.md docs/show/requirements.md
git commit -m "docs(show): document ai show name field in adr, glossary, requirements"
```

---

## Final verification

- [ ] Run the whole affected surface: `go test ./taskgate/internal/show/... ./tests/e2e/show/...` → PASS.
- [ ] `git -C . status` is clean (everything committed).
- [ ] Manual smoke (optional): in a scratch workspace with `.taskgate/human/build` and `.taskgate/shared/deploy/prod`, confirm `taskgate show` aligns summaries and `taskgate ai show` emits `"name"` fields.
