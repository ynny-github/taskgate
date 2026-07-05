# `show` Recursive Listing, Directory-Description Removal, Executable-Only Filter — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `taskgate show` (no argument) list the whole merged tree recursively as an indented tree, remove the `_index` directory-description feature entirely, and hide files that lack an execute bit.

**Architecture:** All logic lives in `taskgate/internal/show`. The resolution layer (`mergedview.go`) gains an executable-only filter, drops `_index` special-casing, and stops loading directory annotations; a new recursive walker `ResolveTree` produces a depth-first flat `[]Entry` with a `Depth` field. The render layer gains an indented-tree human renderer and trims the directory AI envelope. `show.go` rewires the no-argument path to the recursive walker. Tests and docs are updated to match.

**Tech Stack:** Go 1.x, Ginkgo/Gomega (E2E under `tests/e2e/show`), standard `testing` (unit tests in `taskgate/internal/show`), golden files under `tests/e2e/show/testdata/golden`.

## Global Constraints

- Output language for all deliverables (comments, docs, CLI strings): **English** (project `.claude/CLAUDE.md`).
- Conventional Commits for every commit (`.claude/rules/git-commit.md`): `<type>(<scope>): <subject>`, imperative subject, ≤72 chars.
- `go` runs on the host through agent-sandbox; use repo-relative paths. Unit tests: `go test ./taskgate/internal/show/...`. E2E: `go test ./tests/e2e/show/...` (the suite builds the binary via `go build -o <tmp> ./taskgate`).
- Executable bit test: a regular file qualifies iff `mode & 0o111 != 0`.
- Merged-view semantics unchanged: at every level, scan the audience bucket (`human`/`ai`) and `shared`; a name present in both is a collision (FR-013).
- **Scope:** `show` only. The `validate` subcommand's separate `_index` handling (`taskgate/internal/validate/walk.go`) is intentionally **out of scope** for this plan (see "Out of scope / follow-up").

---

## File Structure

**Modified — production:**
- `taskgate/internal/show/mergedview.go` — remove `_index` special-casing, remove `loadAnnotationFor` (dead), stop loading directory annotations, add executable-only filter, add `Entry.Depth`, add `ResolveTree`.
- `taskgate/internal/show/render_human.go` — add `RenderHumanTree` + `writeTreeRow`, rewrite `RenderHumanDirectory` (no summary/body), delete dead `RenderHumanListing`/`writeListingRow`/`displayPath`.
- `taskgate/internal/show/render_ai.go` — drop `Summary`/`Body` from `directoryEnvelope` and the directory branch of `renderAITarget`.
- `taskgate/internal/show/show.go` — `runRoot` uses `ResolveTree` + `RenderHumanTree`.

**Modified — tests / fixtures:**
- `taskgate/internal/show/mergedview_test.go`, `render_test.go`
- `tests/e2e/show/browse_test.go`, `directory_test.go`, `inspect_test.go` (verify only), `errors_test.go` (verify only), `edges_test.go` (verify only)
- `tests/e2e/testutil/workspace.go` (retire `_index` helpers; make task fixtures executable already covered)
- `tests/e2e/show/testdata/golden/` — delete `dir_with_index`, `dir_runnable_index`, `dir_no_recursion`; rewrite `dir_without_index` (→ `dir_children`); add `browse_recursive`.

**Modified — docs:**
- `docs/show/requirements.md`, `docs/show/glossary.md`, `docs/show/adr/0002-directory-description-filename.md`, new `docs/show/adr/0004-recursive-browse-and-executable-filter.md`.

---

## Task 1: Resolution layer — executable-only filter, drop `_index`, drop directory annotations

**Files:**
- Modify: `taskgate/internal/show/mergedview.go`
- Test: `taskgate/internal/show/mergedview_test.go`

**Interfaces:**
- Consumes: existing `Entry`, `scanBucket`, `scanBucketSegment`, `loadAnnotationForWithNote`, `symlinkEscapeNote`.
- Produces:
  - `func isExecutable(absPath string) bool` — true iff `os.Stat(absPath)` succeeds and `mode & 0o111 != 0` (follows symlinks).
  - `scanBucket` / `scanBucketSegment` now exclude non-executable regular files (a symlink whose `symlinkEscapeNote` is non-empty bypasses the filter and stays listed).
  - Directories always resolve to an empty `annotation.AnnotationBlock`.
  - `_index` is no longer special anywhere in this package.

- [ ] **Step 1: Update unit-test fixtures to write executable task files**

In `mergedview_test.go`, change `writeTaskFixture` to mark the file executable, and make the two raw executable-content fixtures executable. Replace the body of `writeTaskFixture` (currently ending in `writeFixture(...)`) so it chmods 0755:

```go
func writeTaskFixture(t *testing.T, ws, rel, summary, body string) {
	t.Helper()
	parts := []string{"#!/bin/sh", "# ---"}
	if summary != "" {
		parts = append(parts, "# summary: "+summary)
	}
	if body != "" {
		parts = append(parts, "# body: |")
		for _, line := range strings.Split(body, "\n") {
			parts = append(parts, "#   "+line)
		}
	}
	parts = append(parts, "# ---", "echo hi", "")
	writeFixture(t, ws, rel, strings.Join(parts, "\n"))
	if err := os.Chmod(filepath.Join(ws, rel), 0o755); err != nil {
		t.Fatal(err)
	}
}
```

In `TestResolveRoot_HumanMergesSharedAndHuman`, the bare-task line writes a non-executable file; make it executable:

```go
	writeFixture(t, tmp, ".taskgate/shared/test", "#!/bin/sh\necho hi\n") // no annotation
	if err := os.Chmod(filepath.Join(tmp, ".taskgate/shared/test"), 0o755); err != nil {
		t.Fatal(err)
	}
```

- [ ] **Step 2: Rewrite the `_index` and unreadable unit tests to the new behavior**

Replace `TestResolveName_Directory_WithIndex` and `TestResolveName_Directory_IndexExecutableStillTreatedAsDescription` and `TestResolveName_Directory_IndexExcludedFromChildren` with the following (a directory carries no annotation; an executable `_index` is now an ordinary child; a non-executable `_index` is filtered out like any non-exec file):

```go
func TestResolveName_Directory_HasNoAnnotation(t *testing.T) {
	tmp := t.TempDir()
	// A non-executable _index is now just a non-executable file: filtered out.
	writeFixture(t, tmp, ".taskgate/human/deploy/_index", "# ---\n# summary: Promote.\n# ---\n")
	writeTaskFixture(t, tmp, ".taskgate/human/deploy/prod", "Prod.", "")
	ws := filepath.Join(tmp, ".taskgate")

	target, _, nf, err := ResolveName(AudienceHuman, ws, "deploy")
	if err != nil {
		t.Fatal(err)
	}
	if nf != nil {
		t.Fatalf("not found: %+v", nf)
	}
	if target.Kind != EntryKindDirectory {
		t.Fatalf("kind = %v, want directory", target.Kind)
	}
	if target.Entry.Annotation != (annotation.AnnotationBlock{}) {
		t.Errorf("directory must carry no annotation, got %+v", target.Entry.Annotation)
	}
	for _, c := range target.Children {
		if c.Name == "_index" {
			t.Error("non-executable _index must be filtered from children")
		}
	}
}

func TestResolveName_Directory_ExecutableIndexIsOrdinaryChild(t *testing.T) {
	tmp := t.TempDir()
	idx := filepath.Join(tmp, ".taskgate/human/deploy/_index")
	if err := os.MkdirAll(filepath.Dir(idx), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(idx, []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTaskFixture(t, tmp, ".taskgate/human/deploy/prod", "", "")
	ws := filepath.Join(tmp, ".taskgate")

	target, _, _, err := ResolveName(AudienceHuman, ws, "deploy")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, c := range target.Children {
		names = append(names, c.Name)
	}
	found := false
	for _, n := range names {
		if n == "_index" {
			found = true
		}
	}
	if !found {
		t.Errorf("executable _index must appear as an ordinary child, got %v", names)
	}
}
```

Replace `TestResolveRoot_UnreadableTaskFile` so it uses an **executable-but-unreadable** file (`0o111`), which stays listed with a note, and `TestResolveRoot_StrayIndexInBucket` so it documents the new reason (non-executable → filtered):

```go
func TestResolveRoot_UnreadableExecutableFile(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses file permissions; this test only runs for unprivileged users")
	}
	tmp := t.TempDir()
	writeTaskFixture(t, tmp, ".taskgate/human/readable", "Readable.", "")
	unreadable := filepath.Join(tmp, ".taskgate/human/unreadable")
	if err := os.MkdirAll(filepath.Dir(unreadable), 0o755); err != nil {
		t.Fatal(err)
	}
	// Executable so it survives the exec filter, but unreadable so the
	// annotation read fails and surfaces a note.
	if err := os.WriteFile(unreadable, []byte("#!/bin/sh\necho hi\n"), 0o111); err != nil {
		t.Fatal(err)
	}

	entries, col := resolveRoot(t, tmp, AudienceHuman)
	if col != nil {
		t.Fatalf("unexpected collision: %+v", col)
	}
	names := map[string]Entry{}
	for _, e := range entries {
		names[e.Name] = e
	}
	if _, ok := names["unreadable"]; !ok {
		t.Fatal("executable-but-unreadable entry should still surface")
	}
	if names["unreadable"].Note == "" {
		t.Errorf("Note should describe the read failure")
	}
}

func TestResolveRoot_NonExecutableFileHidden(t *testing.T) {
	tmp := t.TempDir()
	writeTaskFixture(t, tmp, ".taskgate/human/build", "Build.", "")
	writeFixture(t, tmp, ".taskgate/human/notes.txt", "just notes\n") // 0644, non-exec
	// stray non-executable _index: also hidden, now purely because it is non-exec.
	writeFixture(t, tmp, ".taskgate/human/_index", "---\nsummary: bucket\n---\n")

	entries, col := resolveRoot(t, tmp, AudienceHuman)
	if col != nil {
		t.Fatalf("unexpected collision: %+v", col)
	}
	for _, e := range entries {
		if e.Name == "notes.txt" || e.Name == "_index" {
			t.Errorf("non-executable file %q must be hidden", e.Name)
		}
	}
	if len(entries) != 1 || entries[0].Name != "build" {
		t.Errorf("want only executable 'build', got %v", entryNames(entries))
	}
}
```

Delete the now-replaced `TestResolveRoot_UnreadableTaskFile` and `TestResolveRoot_StrayIndexInBucket` bodies.

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./taskgate/internal/show/... -run 'Directory_HasNoAnnotation|ExecutableIndex|NonExecutableFileHidden|UnreadableExecutable'`
Expected: FAIL / does not compile (filter + behavior not implemented yet).

- [ ] **Step 4: Implement the resolution changes**

In `mergedview.go`:

(a) Delete the `indexFilename` const and its doc comment (lines ~12–13).

(b) In `scanBucket`, remove the `if name == indexFilename { continue }` block and add the executable filter after computing `ann, note`:

```go
	for _, de := range dirEntries {
		name := de.Name()
		absChild := filepath.Join(absDir, name)
		kind := EntryKindTask
		if de.IsDir() {
			kind = EntryKindDirectory
		}
		ann, note := loadAnnotationForWithNote(absChild, kind)
		if kind == EntryKindTask && note == "" && !isExecutable(absChild) {
			// Non-executable files cannot be run; hide them (FR-014).
			// An escaping/broken symlink (note != "") is left listed.
			continue
		}
		out[name] = Entry{
			Name:       name,
			Path:       bucketRelPath(bucket, filepath.Join(sub, name)),
			Kind:       kind,
			Annotation: ann,
			Note:       note,
		}
	}
```

(c) In `scanBucketSegment`, remove the `if seg == indexFilename { return nil, nil }` guard and add the executable filter before returning the Entry:

```go
	kind := EntryKindTask
	if info.IsDir() {
		kind = EntryKindDirectory
	}
	ann, note := loadAnnotationForWithNote(abs, kind)
	if kind == EntryKindTask && note == "" && !isExecutable(abs) {
		return nil, nil // non-executable file: not resolvable (FR-014)
	}
	return &Entry{
		Name:       seg,
		Path:       bucketRelPath(bucket, filepath.Join(sub, seg)),
		Kind:       kind,
		Annotation: ann,
		Note:       note,
	}, nil
```

(d) Delete the dead `loadAnnotationFor` function (the non-note wrapper). Rewrite `loadAnnotationForWithNote` so directories carry no annotation and files are read directly (no `_index`):

```go
// loadAnnotationForWithNote returns a task file's annotation plus a
// non-fatal note describing any read failure. Directories carry no
// annotation. An escaping or broken symlink yields an empty block and a
// descriptive note (FR-008).
func loadAnnotationForWithNote(absPath string, kind EntryKind) (annotation.AnnotationBlock, string) {
	if note := symlinkEscapeNote(absPath); note != "" {
		return annotation.AnnotationBlock{}, note
	}
	if kind == EntryKindDirectory {
		return annotation.AnnotationBlock{}, ""
	}
	f, err := os.Open(absPath)
	if err != nil {
		if os.IsPermission(err) {
			return annotation.AnnotationBlock{}, "permission denied"
		}
		return annotation.AnnotationBlock{}, ""
	}
	defer f.Close()
	block, err := annotation.Parse(f)
	if err != nil {
		return annotation.AnnotationBlock{}, ""
	}
	return block, ""
}

// isExecutable reports whether absPath is an executable file. Symlinks are
// followed to their target's mode — the same file taskgate run executes.
func isExecutable(absPath string) bool {
	info, err := os.Stat(absPath)
	if err != nil {
		return false
	}
	return info.Mode()&0o111 != 0
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./taskgate/internal/show/...`
Expected: PASS (all mergedview tests, including the rewritten ones).

- [ ] **Step 6: Commit**

```bash
git add taskgate/internal/show/mergedview.go taskgate/internal/show/mergedview_test.go
git commit -m "feat(show): filter non-executable files and drop _index handling

What: scanBucket/scanBucketSegment now hide non-executable regular files
and no longer treat _index specially; directories carry no annotation.
Why: taskgate run only runs executable tasks, and the directory-
description feature is being removed."
```

---

## Task 2: Recursive tree resolver

**Files:**
- Modify: `taskgate/internal/show/mergedview.go` (add `Entry.Depth`, `ResolveTree`)
- Test: `taskgate/internal/show/mergedview_test.go`

**Interfaces:**
- Consumes: `scanBucket`, `mergeLevel`, `EntrySlice`/`LessEntries`.
- Produces:
  - `Entry` gains `Depth int` (0 at the merged root; +1 per level).
  - `func ResolveTree(audience Audience, workspaceDir string) ([]Entry, *CollisionReport, error)` — depth-first pre-order flat slice of every entry; a directory row is immediately followed by its descendants. A collision at any visited level returns `(nil, report, nil)`.

- [ ] **Step 1: Write the failing test**

Add to `mergedview_test.go`:

```go
func TestResolveTree_RecursesDepthFirst(t *testing.T) {
	tmp := t.TempDir()
	writeTaskFixture(t, tmp, ".taskgate/human/build", "Build.", "")
	writeTaskFixture(t, tmp, ".taskgate/shared/deploy/prod", "Prod.", "")
	writeTaskFixture(t, tmp, ".taskgate/shared/deploy/stg", "Stg.", "")
	ws := filepath.Join(tmp, ".taskgate")

	entries, col, err := ResolveTree(AudienceHuman, ws)
	if err != nil {
		t.Fatal(err)
	}
	if col != nil {
		t.Fatalf("unexpected collision: %+v", col)
	}
	type row struct {
		name  string
		depth int
	}
	got := make([]row, len(entries))
	for i, e := range entries {
		got[i] = row{e.Name, e.Depth}
	}
	want := []row{
		{"deploy", 0}, // directories first at the root
		{"prod", 1},
		{"stg", 1},
		{"build", 0},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d rows %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d = %+v, want %+v (full: %v)", i, got[i], want[i], got)
		}
	}
}

func TestResolveTree_CollisionAtDepth(t *testing.T) {
	tmp := t.TempDir()
	writeTaskFixture(t, tmp, ".taskgate/human/deploy/prod", "h.", "")
	writeTaskFixture(t, tmp, ".taskgate/shared/deploy/prod", "s.", "")
	ws := filepath.Join(tmp, ".taskgate")

	_, col, err := ResolveTree(AudienceHuman, ws)
	if err != nil {
		t.Fatal(err)
	}
	if col == nil {
		t.Fatal("expected a collision report from a deep collision")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./taskgate/internal/show/... -run ResolveTree`
Expected: FAIL — `ResolveTree` undefined.

- [ ] **Step 3: Implement `Entry.Depth` and `ResolveTree`**

In `mergedview.go`, add `Depth` to the `Entry` struct:

```go
type Entry struct {
	Name       string
	Path       string
	Kind       EntryKind
	Annotation annotation.AnnotationBlock
	Note       string
	// Depth is the entry's level in a recursive listing (0 at the merged
	// root). Set by ResolveTree; zero for single-level resolutions.
	Depth int
}
```

Add, near `ResolveRoot`:

```go
// ResolveTree walks the entire merged view depth-first and returns a flat,
// pre-order slice: each directory row is immediately followed by its
// descendants. Entry.Depth records each row's level (root = 0). A collision
// at any visited level short-circuits with (nil, report, nil).
func ResolveTree(audience Audience, workspaceDir string) ([]Entry, *CollisionReport, error) {
	return walkTree(audience, workspaceDir, "", 0)
}

func walkTree(audience Audience, workspaceDir, sub string, depth int) ([]Entry, *CollisionReport, error) {
	aud, err := scanBucket(workspaceDir, audience.Bucket(), sub)
	if err != nil {
		return nil, nil, err
	}
	sh, err := scanBucket(workspaceDir, "shared", sub)
	if err != nil {
		return nil, nil, err
	}
	merged, col := mergeLevel(aud, sh)
	if col != nil {
		return nil, col, nil
	}
	var out []Entry
	for _, e := range merged {
		e.Depth = depth
		out = append(out, e)
		if e.Kind != EntryKindDirectory {
			continue
		}
		childSub := e.Name
		if sub != "" {
			childSub = sub + "/" + e.Name
		}
		children, cCol, err := walkTree(audience, workspaceDir, childSub, depth+1)
		if err != nil {
			return nil, nil, err
		}
		if cCol != nil {
			return nil, cCol, nil
		}
		out = append(out, children...)
	}
	return out, nil, nil
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./taskgate/internal/show/... -run ResolveTree`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add taskgate/internal/show/mergedview.go taskgate/internal/show/mergedview_test.go
git commit -m "feat(show): add recursive merged-view walker

What: ResolveTree returns a depth-first flat slice of every merged-view
entry with a Depth field, checking collisions at every level.
Why: the no-argument show will list the whole tree, not just the root."
```

---

## Task 3: Render layer — human tree, directory rewrite, AI envelope trim

**Files:**
- Modify: `taskgate/internal/show/render_human.go`, `taskgate/internal/show/render_ai.go`
- Test: `taskgate/internal/show/render_test.go`

**Interfaces:**
- Consumes: `Entry` (with `Depth`), `ResolvedTarget`.
- Produces:
  - `func RenderHumanTree(w io.Writer, entries []Entry) error` — indented tree; two spaces per `Depth`. Directory row: `<indent><name>/`. Task row: `<indent><name>` plus `\t<summary>` when a trimmed summary is present.
  - `func writeTreeRow(w io.Writer, e Entry, depth int) error` — shared row writer.
  - `RenderHumanDirectory` — emits the directory's real path, a blank line, then immediate children as one-level (`depth 1`) tree rows. No summary/body section.
  - `directoryEnvelope` — no longer has `Summary` or `Body`.
- Removed: `RenderHumanListing`, `writeListingRow`, `displayPath` (dead after Task 4 rewires; delete here since `RenderHumanDirectory` stops using `writeListingRow`). `RenderHumanListing` is still referenced by `show.go` until Task 4 — **keep `RenderHumanListing` and `writeListingRow` for now**; delete them in Task 4 after rewiring.

- [ ] **Step 1: Replace the directory render tests and add tree-render tests**

In `render_test.go`, replace `TestRenderHumanDirectory_FullPayload`, `TestRenderHumanDirectory_NoIndex`, and `TestRenderHumanDirectory_NoChildren` with tests for the new no-summary/body directory shape, and add `RenderHumanTree` tests:

```go
func TestRenderHumanDirectory_PathThenChildren(t *testing.T) {
	target := ResolvedTarget{
		Kind:  EntryKindDirectory,
		Entry: Entry{Path: ".taskgate/human/deploy", Kind: EntryKindDirectory},
		Children: []Entry{
			{Name: "sub", Path: ".taskgate/human/deploy/sub", Kind: EntryKindDirectory},
			{Name: "canary", Path: ".taskgate/human/deploy/canary", Kind: EntryKindTask,
				Annotation: annotation.AnnotationBlock{Summary: "Canary."}},
			{Name: "prod", Path: ".taskgate/human/deploy/prod", Kind: EntryKindTask},
		},
	}
	var buf bytes.Buffer
	if err := RenderHumanDirectory(&buf, target); err != nil {
		t.Fatal(err)
	}
	want := ".taskgate/human/deploy\n\n  sub/\n  canary\tCanary.\n  prod\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

func TestRenderHumanTree_IndentsByDepth(t *testing.T) {
	entries := []Entry{
		{Name: "deploy", Kind: EntryKindDirectory, Depth: 0},
		{Name: "prod", Kind: EntryKindTask, Depth: 1,
			Annotation: annotation.AnnotationBlock{Summary: "Prod."}},
		{Name: "build", Kind: EntryKindTask, Depth: 0,
			Annotation: annotation.AnnotationBlock{Summary: "Build."}},
		{Name: "bare", Kind: EntryKindTask, Depth: 0},
	}
	var buf bytes.Buffer
	if err := RenderHumanTree(&buf, entries); err != nil {
		t.Fatal(err)
	}
	want := "deploy/\n  prod\tProd.\nbuild\tBuild.\nbare\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}
```

Add an assertion that the directory AI envelope has no `summary`/`body`. Replace `TestRenderAI_Directory` with:

```go
func TestRenderAI_Directory(t *testing.T) {
	target := ResolvedTarget{
		Kind:  EntryKindDirectory,
		Entry: Entry{Path: ".taskgate/shared/deploy", Kind: EntryKindDirectory},
		Children: []Entry{
			{Name: "prod", Path: ".taskgate/shared/deploy/prod", Kind: EntryKindTask,
				Annotation: annotation.AnnotationBlock{Summary: "Prod."}},
		},
	}
	var buf bytes.Buffer
	if err := renderAITarget(&buf, target); err != nil {
		t.Fatal(err)
	}
	got := decodeOneJSON(t, buf.Bytes())
	if got["kind"] != "directory" {
		t.Errorf("kind = %v", got["kind"])
	}
	if _, present := got["summary"]; present {
		t.Errorf("directory envelope must not carry summary: %v", got)
	}
	if _, present := got["body"]; present {
		t.Errorf("directory envelope must not carry body: %v", got)
	}
	rows, ok := got["entries"].([]any)
	if !ok || len(rows) != 1 {
		t.Fatalf("entries = %v", got["entries"])
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./taskgate/internal/show/... -run 'RenderHumanDirectory_PathThenChildren|RenderHumanTree|RenderAI_Directory'`
Expected: FAIL — `RenderHumanTree` undefined and directory renderers still emit summary/body.

- [ ] **Step 3: Implement the renderers**

In `render_human.go`, add the tree writer and rewrite `RenderHumanDirectory` (leave `RenderHumanListing`/`writeListingRow`/`displayPath` in place for now):

```go
// writeTreeRow writes a single indented tree row: two spaces per depth,
// then the basename. Directories get a trailing "/"; task rows append a
// tab and the trimmed summary when one is present.
func writeTreeRow(w io.Writer, e Entry, depth int) error {
	indent := strings.Repeat("  ", depth)
	if e.Kind == EntryKindDirectory {
		_, err := fmt.Fprintf(w, "%s%s/\n", indent, e.Name)
		return err
	}
	summary := strings.TrimSpace(e.Annotation.Summary)
	if summary == "" {
		_, err := fmt.Fprintf(w, "%s%s\n", indent, e.Name)
		return err
	}
	_, err := fmt.Fprintf(w, "%s%s\t%s\n", indent, e.Name, summary)
	return err
}

// RenderHumanTree writes the recursive listing as an indented tree, one row
// per entry, indented by Entry.Depth.
func RenderHumanTree(w io.Writer, entries []Entry) error {
	for _, e := range entries {
		if err := writeTreeRow(w, e, e.Depth); err != nil {
			return err
		}
	}
	return nil
}
```

Replace the body of `RenderHumanDirectory` with:

```go
// RenderHumanDirectory writes the directory-target view: the directory's
// real path, a blank line, then its immediate children as one-level tree
// rows. Directories carry no summary/body.
func RenderHumanDirectory(w io.Writer, target ResolvedTarget) error {
	if _, err := fmt.Fprintln(w, target.Entry.Path); err != nil {
		return err
	}
	if len(target.Children) > 0 {
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
		for _, c := range target.Children {
			if err := writeTreeRow(w, c, 1); err != nil {
				return err
			}
		}
	}
	return nil
}
```

In `render_ai.go`, drop `Summary` and `Body` from `directoryEnvelope`:

```go
type directoryEnvelope struct {
	Kind     string        `json:"kind"`
	Path     string        `json:"path"`
	Audience string        `json:"audience"`
	Entries  []childRecord `json:"entries"`
}
```

And in `renderAITarget`'s directory branch, drop the removed fields:

```go
	case EntryKindDirectory:
		env := directoryEnvelope{
			Kind:     "directory",
			Path:     target.Entry.Path,
			Audience: "ai",
			Entries:  childRecords(target.Children),
		}
		return renderAI(w, env)
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./taskgate/internal/show/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add taskgate/internal/show/render_human.go taskgate/internal/show/render_ai.go taskgate/internal/show/render_test.go
git commit -m "feat(show): add indented tree renderer, drop directory descriptions

What: adds RenderHumanTree/writeTreeRow, rewrites RenderHumanDirectory to
path-header + one-level tree, and removes summary/body from the directory
AI envelope.
Why: recursive browse renders as a tree and directories no longer carry a
description."
```

---

## Task 4: Wire the no-argument path to the recursive walker

**Files:**
- Modify: `taskgate/internal/show/show.go`
- Modify: `taskgate/internal/show/render_human.go` (delete dead `RenderHumanListing`, `writeListingRow`, `displayPath`)
- Test: `taskgate/internal/show/render_test.go` (delete `RenderHumanListing` tests)

**Interfaces:**
- Consumes: `ResolveTree`, `RenderHumanTree`, `renderAIRootListing`, `emitNotices`.
- Produces: `runRoot` lists the whole tree; human form uses `RenderHumanTree`, AI form reuses `renderAIRootListing` over the flat tree slice.

- [ ] **Step 1: Rewire `runRoot`**

In `show.go`, replace `runRoot`:

```go
func runRoot(audience Audience, ws string, stdout, stderr io.Writer) (int, error) {
	entries, col, err := ResolveTree(audience, ws)
	if err != nil {
		return ExitGeneric, err
	}
	if col != nil {
		return renderCollision(audience, *col, stdout, stderr)
	}
	if audience == AudienceAI {
		return ExitSuccess, renderAIRootListing(stdout, entries)
	}
	emitNotices(stderr, entries)
	return ExitSuccess, RenderHumanTree(stdout, entries)
}
```

- [ ] **Step 2: Delete dead render helpers and their tests**

In `render_human.go`, delete `RenderHumanListing`, `writeListingRow`, and `displayPath` (no longer referenced). In `render_test.go`, delete `TestRenderHumanListing_BasicRows`, `TestRenderHumanListing_NoSummaryPathOnly`, and `TestRenderHumanListing_Empty`.

- [ ] **Step 3: Run to verify pass and no dead references**

Run: `go build ./... && go test ./taskgate/internal/show/...`
Expected: PASS, no "declared and not used" or undefined-symbol errors.

- [ ] **Step 4: Commit**

```bash
git add taskgate/internal/show/show.go taskgate/internal/show/render_human.go taskgate/internal/show/render_test.go
git commit -m "feat(show): list the whole tree when no name is given

What: runRoot now walks the merged view recursively (ResolveTree) and
renders an indented tree (human) or a flat listing envelope (AI); removes
the now-dead flat-listing human renderer.
Why: no-argument show should present the entire workspace, not just root."
```

---

## Task 5: E2E suite and fixtures

**Files:**
- Modify: `tests/e2e/show/browse_test.go`, `tests/e2e/show/directory_test.go`
- Modify: `tests/e2e/testutil/workspace.go`
- Delete: `tests/e2e/show/testdata/golden/dir_with_index.golden`, `dir_runnable_index.golden`, `dir_no_recursion.golden`
- Rewrite: `tests/e2e/show/testdata/golden/dir_without_index.golden` → new file `dir_children.golden`
- Create: `tests/e2e/show/testdata/golden/browse_recursive.golden`
- Verify only (expected to still pass): `tests/e2e/show/inspect_test.go`, `errors_test.go`, `edges_test.go`

**Interfaces:**
- Consumes: `testutil.Workspace`, `MatchGolden`, `Lines`, `Cols`.

- [ ] **Step 1: Add the recursive golden fixture**

Create `tests/e2e/show/testdata/golden/browse_recursive.golden` with exactly (note the trailing newline):

```
deploy/
  prod	Prod.
  stg	Stg.
build	Build.
```

Create `tests/e2e/show/testdata/golden/dir_children.golden` with exactly:

```
.taskgate/human/deploy

  canary	Promote to canary.
  prod	Promote to production.
```

- [ ] **Step 2: Rewrite `browse_test.go` root-listing expectations**

Replace the first `Describe` ("FR-001 — root browse") body so the no-argument output is the recursive indented tree, and add a recursion case. The human bucket paths collapse to basenames in the tree:

```go
var _ = Describe("taskgate show: no-argument browse lists the whole tree", func() {
	var ws *testutil.Workspace

	BeforeEach(func() {
		ws = testutil.New(GinkgoT().TempDir(), TaskgateBinary)
	})

	Context("nested tasks across buckets", func() {
		It("renders a depth-first indented tree", func() {
			ws.WriteAnnotatedTask(".taskgate/human/build", "Build.", "")
			ws.WriteAnnotatedTask(".taskgate/shared/deploy/prod", "Prod.", "")
			ws.WriteAnnotatedTask(".taskgate/shared/deploy/stg", "Stg.", "")
			out := ws.Run("show")
			Expect(out.ExitCode).To(Equal(0))
			Expect(out.Stderr).To(BeEmpty())
			Expect(out.Stdout).To(testutil.MatchGolden("browse_recursive"))
		})
	})
})
```

Update the "unannotated tasks still appear" context to expect a basename tree row (`bare`, not the full path):

```go
		It("appears with basename only, no error", func() {
			ws.WriteBareTask(".taskgate/shared/bare")
			out := ws.Run("show")
			Expect(out.ExitCode).To(Equal(0))
			Expect(out.Stderr).To(BeEmpty())
			Expect(out.Stdout).To(Equal(testutil.Lines("bare", "")))
		})
```

The `ai show` no-argument context in `browse_test.go` still asserts full paths via `ContainSubstring` — those hold (AI form keeps full paths). Leave it, but note it now also exercises recursion; no change required.

- [ ] **Step 3: Rewrite `directory_test.go`**

Replace the entire file with tests that reflect the new directory behavior (no `_index`, path-header + one-level tree, executable-only), keeping the many-children and AI-envelope coverage:

```go
package show_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/ynny-github/taskgate/tests/e2e/testutil"
)

var _ = Describe("taskgate show: directory lists its immediate children", func() {
	var ws *testutil.Workspace

	BeforeEach(func() {
		ws = testutil.New(GinkgoT().TempDir(), TaskgateBinary)
	})

	Context("directory with task children", func() {
		It("shows path header then a one-level tree, no summary/body", func() {
			ws.WriteAnnotatedTask(".taskgate/human/deploy/canary", "Promote to canary.", "")
			ws.WriteAnnotatedTask(".taskgate/human/deploy/prod", "Promote to production.", "")
			out := ws.Run("show", "deploy")
			Expect(out.ExitCode).To(Equal(0))
			Expect(out.Stderr).To(BeEmpty())
			Expect(out.Stdout).To(testutil.MatchGolden("dir_children"))
		})
	})
})

var _ = Describe("taskgate show: directory listing is one level deep", func() {
	var ws *testutil.Workspace

	BeforeEach(func() {
		ws = testutil.New(GinkgoT().TempDir(), TaskgateBinary)
	})

	Context("nested sub-directory", func() {
		It("appears as a single row, not expanded", func() {
			ws.WriteAnnotatedTask(".taskgate/human/deploy/prod/run", "Run a prod deploy.", "")
			out := ws.Run("show", "deploy")
			Expect(out.ExitCode).To(Equal(0))
			Expect(out.Stderr).To(BeEmpty())
			Expect(out.Stdout).To(Equal(testutil.Lines(
				".taskgate/human/deploy",
				"",
				"  prod/",
				"",
			)))
		})
	})
})

var _ = Describe("taskgate show: non-executable files are hidden", func() {
	var ws *testutil.Workspace

	BeforeEach(func() {
		ws = testutil.New(GinkgoT().TempDir(), TaskgateBinary)
	})

	Context("a directory with an executable and a non-executable file", func() {
		It("lists only the executable one", func() {
			ws.WriteAnnotatedTask(".taskgate/human/tools/run", "Runnable.", "")
			ws.WriteFile(".taskgate/human/tools/notes.txt", "just notes\n", false)
			out := ws.Run("show", "tools")
			Expect(out.ExitCode).To(Equal(0))
			Expect(out.Stdout).To(ContainSubstring("run"))
			Expect(out.Stdout).NotTo(ContainSubstring("notes.txt"))
		})
	})

	Context("a named non-executable file", func() {
		It("is not found", func() {
			ws.WriteFile(".taskgate/human/notes.txt", "just notes\n", false)
			out := ws.Run("show", "notes.txt")
			Expect(out.ExitCode).To(Equal(3))
			Expect(out.Stderr).To(ContainSubstring("not found"))
		})
	})
})

var _ = Describe("taskgate show: no truncation with many children", func() {
	var ws *testutil.Workspace

	BeforeEach(func() {
		ws = testutil.New(GinkgoT().TempDir(), TaskgateBinary)
	})

	Context("50 children", func() {
		It("all appear in listing without truncation", func() {
			ws.WriteManyBareTasks(".taskgate/human/many", 50)
			out := ws.Run("show", "many")
			Expect(out.ExitCode).To(Equal(0))
			Expect(out.Stderr).To(BeEmpty())
			Expect(out.Stdout).To(ContainSubstring("child00"))
			Expect(out.Stdout).To(ContainSubstring("child49"))
		})
	})
})

var _ = Describe("taskgate ai show: directory envelope shape", func() {
	var ws *testutil.Workspace

	BeforeEach(func() {
		ws = testutil.New(GinkgoT().TempDir(), TaskgateBinary)
	})

	Context("directory envelope", func() {
		It("has child entries and no summary/body", func() {
			ws.WriteAnnotatedTask(".taskgate/shared/deploy/canary", "Promote to canary.", "")
			ws.WriteAnnotatedTask(".taskgate/shared/deploy/prod", "Promote to production.", "")
			out := ws.Run("ai", "show", "deploy")
			Expect(out.ExitCode).To(Equal(0))
			Expect(out.Stderr).To(BeEmpty())

			var envelope map[string]interface{}
			Expect(json.Unmarshal([]byte(out.Stdout), &envelope)).To(Succeed())
			Expect(envelope["kind"]).To(Equal("directory"))
			Expect(envelope["path"]).To(Equal(".taskgate/shared/deploy"))
			Expect(envelope).NotTo(HaveKey("summary"))
			Expect(envelope).NotTo(HaveKey("body"))
			Expect(out.Stdout).To(ContainSubstring(".taskgate/shared/deploy/canary"))
			Expect(out.Stdout).To(ContainSubstring(".taskgate/shared/deploy/prod"))
		})
	})
})
```

- [ ] **Step 4: Retire the `_index` fixture helpers**

In `tests/e2e/testutil/workspace.go`, delete `WriteIndex`, `WriteRunnableIndex`, `WriteMalformedIndex` (they existed only for the directory-description feature and are no longer referenced). Leave `WriteFile`, `WriteAnnotatedTask`, `WriteBareTask`, `MakeUnreadable`, `Symlink`, `WriteManyBareTasks`, `WriteLeadingCommentsTask` intact.

- [ ] **Step 5: Delete obsolete golden files**

```bash
git rm tests/e2e/show/testdata/golden/dir_with_index.golden \
       tests/e2e/show/testdata/golden/dir_runnable_index.golden \
       tests/e2e/show/testdata/golden/dir_no_recursion.golden \
       tests/e2e/show/testdata/golden/dir_without_index.golden
```

- [ ] **Step 6: Run the full E2E suite**

Run: `go test ./tests/e2e/show/...`
Expected: PASS. `inspect_test.go`, `errors_test.go`, and `edges_test.go` are unchanged and must stay green (the unreadable-file edge now passes because the 0000 file is filtered, and the other entry still appears; the symlink-escape edge still lists the escapee because escape notes bypass the filter).

If `edges_test.go`'s "unreadable file" spec fails because it asserts anything about the locked file appearing, adjust it to only assert the sibling `lint` entry appears (it currently does only that — no change expected).

- [ ] **Step 7: Commit**

```bash
git add tests/e2e/ 
git commit -m "test(show): cover recursive browse, one-level directory, exec filter

What: rewrites browse/directory E2E specs and goldens for the recursive
tree, path-header directory view, and executable-only filtering; retires
the _index fixture helpers and obsolete goldens.
Why: match the reshaped show behavior."
```

---

## Task 6: Documentation

**Files:**
- Modify: `docs/show/requirements.md`, `docs/show/glossary.md`, `docs/show/adr/0002-directory-description-filename.md`
- Create: `docs/show/adr/0004-recursive-browse-and-executable-filter.md`

**Interfaces:** none (docs only).

- [ ] **Step 1: Update `requirements.md`**

Apply these edits:
- **FR-003b**: replace with — "When the name resolves to a **directory**, system MUST output that directory's real physical path, then a one-line entry for each **immediate** child in the merged view (task or sub-directory). Directories carry no summary or body."
- **FR-003c**: replace with — "When **no name argument** is given, system MUST present the **merged audience-filtered view recursively**: every entry at every level, depth-first, ordered per FR-007 within each level. Bucket directories MUST NOT appear as rows."
- **FR-004**: delete (directory description file no longer exists).
- **FR-010**: replace with — "System MUST NOT recurse beyond a directory's immediate children **when a directory is named explicitly**. The no-argument case (FR-003c) is the sole recursive form."
- **FR-011**: delete (`_index` is no longer reserved).
- Add **FR-014**: "System MUST exclude regular files without an execute bit (`mode & 0o111 == 0`) from both listing and name resolution; a named non-executable file resolves as not-found. Directories are listed regardless of their execute bit. Symlinks are judged by their resolved target's mode; a symlink whose target escapes `.taskgate/` retains FR-008 handling and is not evaluated for the execute bit."
- In the intro paragraph, correct the test pointer from `tests/features/show/*.feature` (pytest / `taskgate run e2e`) to the actual suite: `tests/e2e/show/*_test.go` (Ginkgo, run via `go test ./tests/e2e/show/...`).

- [ ] **Step 2: Update `glossary.md`**

- **Task entry**: keep "A single executable file …" (already correct); remove "may carry an annotation … A task's physical location …" only if inaccurate — leave as is.
- **Directory entry**: replace the sentence "Has a path; may carry a summary and body via an optional dedicated description file placed inside it;" with "Has a path; carries no summary or body;".
- Delete the **Directory description file** section entirely.
- **Output record**: replace "its body (only for the single-target file/directory case)" with "its body (only for the single-target file case)" and "for a directory target — the list of immediate child records" stays.

- [ ] **Step 3: Supersede ADR-0002**

At the top of `docs/show/adr/0002-directory-description-filename.md`, change the status line to:

```markdown
**Status**: Superseded by ADR-0004 (2026-07-06) — the directory-description feature was removed; `_index` is now an ordinary file.
```

- [ ] **Step 4: Create ADR-0004**

Create `docs/show/adr/0004-recursive-browse-and-executable-filter.md`:

```markdown
# ADR-0004: Recursive no-argument browse and executable-only listing

**Status**: Accepted (2026-07-06)

## Context

`taskgate show` previously listed only the immediate root level and let
directories carry a summary/body via an `_index` description file. Two
changes were requested: a no-argument `show` should reveal the whole
workspace, and files that cannot be run should not be advertised.

## Decision

1. **No-argument `show` walks the merged view recursively** and renders an
   indented tree (human) or a flat, full-path listing envelope (AI). An
   explicitly named directory still lists only its immediate children.
2. **The `_index` directory-description feature is removed.** Directories
   carry no summary/body, and `_index` loses its reserved status — it is an
   ordinary file, subject to the same rules as any other.
3. **Non-executable regular files are hidden** from listing and name
   resolution (`mode & 0o111 == 0`), matching `taskgate run`, which only
   runs executable tasks. Symlinks are judged by their resolved target's
   mode; escaping symlinks keep their FR-008 handling.

## Consequences

- The merged view has one recursive entry point (no argument) and one
  one-level entry point (explicit directory), which is a small asymmetry
  callers must learn.
- Collision detection now spans every level the recursive walk visits.
- The `validate` subcommand still recognizes `_index`; aligning it is a
  separate follow-up.
```

- [ ] **Step 5: Commit**

```bash
git add docs/show/
git commit -m "docs(show): document recursive browse and executable-only listing

What: revises FR-003b/c, FR-010, adds FR-014, removes FR-004/FR-011 and
the directory-description glossary entry, supersedes ADR-0002, adds
ADR-0004, and corrects the test pointer to the Ginkgo suite.
Why: keep the show spec in sync with the reshaped behavior."
```

---

## Out of scope / follow-up

- **`validate` subcommand `_index` handling** (`taskgate/internal/validate/walk.go`): `validate` still treats `_index` as a directory description and excludes it from name-collision slots. Since the description feature is gone from `show`, `validate` should eventually stop special-casing `_index` too. This is a separate change against a different subsystem and is **not** part of this plan — flag it for a follow-up spec.
- No change to `show <task>` detail output or to `taskgate run` resolution.

---

## Self-Review

**Spec coverage:**
- Recursive no-argument listing → Task 2 (`ResolveTree`) + Task 4 (wire) + Task 5 (E2E `browse_recursive`).
- Human indented tree / AI flat listing → Task 3 + Task 4.
- `show <dir>` one level, no description → Task 3 (`RenderHumanDirectory`) + Task 5 (`dir_children`).
- `_index` removal → Task 1 (resolution) + Task 3/4 (envelope) + Task 5 (fixtures) + Task 6 (docs, ADRs).
- Executable-only filter → Task 1 (`isExecutable`) + Task 5 (E2E) + Task 6 (FR-014).
- Collision across levels → Task 2 (`TestResolveTree_CollisionAtDepth`) + existing `errors_test.go`.
- `show <task>` unchanged → verified by `inspect_test.go` (Task 5 Step 6).
- Test-pointer / glossary / ADR fixes → Task 6.

**Placeholder scan:** none — every code and golden block is literal.

**Type consistency:** `ResolveTree(audience Audience, workspaceDir string) ([]Entry, *CollisionReport, error)`, `walkTree(...)`, `isExecutable(absPath string) bool`, `RenderHumanTree(w io.Writer, entries []Entry) error`, `writeTreeRow(w io.Writer, e Entry, depth int) error`, `Entry.Depth int`, and the trimmed `directoryEnvelope` are referenced consistently across Tasks 1–4.
