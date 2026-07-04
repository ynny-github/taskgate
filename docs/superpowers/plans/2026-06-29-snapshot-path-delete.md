# Snapshot Path/Delete + Hashed Dir Naming Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Name the snapshot directory by a hash of the project path, and add `taskgate snapshot path` / `taskgate snapshot delete` subcommands that take an optional path (defaulting to cwd).

**Architecture:** `snapshotDirFn` (`taskgate/cmd/ai.go`) is the single resolver used by `install` and `ai run`; it gains a pure `snapshotDirName(root)` helper that hashes the project root. Two new subcommands share a `resolveSnapshotDir(args)` helper that picks the working directory (arg or cwd), applies the `snapshotDirOverride` test hook, and calls the resolver.

**Tech Stack:** Go, cobra, standard library (`crypto/sha256`, `encoding/hex`, `os`). Tests use the existing `snapshotDirOverride` hook and `testing` + `t.TempDir()`.

## Global Constraints

- Conventional Commits for every commit (`type(scope): subject`, imperative mood).
- Hash: first 12 hex chars of `sha256(root)` where `root` is the absolute git project root.
- New subcommands accept an optional `path` arg; when omitted, use `os.Getwd()`. `snapshot install` is unchanged (cwd-only, no arg).
- `snapshot delete` never prompts; both outcomes exit 0.
- All human-readable output strings in English.

---

## File Structure

- `taskgate/cmd/ai.go` — Modify `snapshotDirFn`; add pure `snapshotDirName(root string) string`.
- `taskgate/cmd/snapshot.go` — Add `resolveSnapshotDir`, `newSnapshotPathCmd`/`snapshotPath`, `newSnapshotDeleteCmd`/`snapshotDelete`; register both under `newSnapshotCmd`.
- `taskgate/cmd/snapshot_test.go` — Add tests for naming, `path`, and `delete`.

---

### Task 1: Hashed snapshot directory naming

**Files:**
- Modify: `taskgate/cmd/ai.go:18-28` (`snapshotDirFn`), add `snapshotDirName` helper
- Test: `taskgate/cmd/snapshot_test.go`

**Interfaces:**
- Consumes: `detectProjectRoot(cwd) string` (existing)
- Produces: `snapshotDirName(root string) string` — returns the 12-char lowercase-hex directory name for a project root. `snapshotDirFn` unchanged in signature: `func(cwd string) (string, error)`.

- [ ] **Step 1: Write the failing test**

Add to `taskgate/cmd/snapshot_test.go`:

```go
func TestSnapshotDirName(t *testing.T) {
	a := snapshotDirName("/Users/yn/work/taskgate")
	b := snapshotDirName("/Users/yn/other/taskgate")

	if len(a) != 12 {
		t.Errorf("expected 12-char name, got %q (len %d)", a, len(a))
	}
	if a != snapshotDirName("/Users/yn/work/taskgate") {
		t.Error("expected stable name for the same root")
	}
	if a == b {
		t.Error("expected different names for roots that share a basename")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd taskgate && go test ./cmd/ -run TestSnapshotDirName`
Expected: FAIL — `undefined: snapshotDirName`

- [ ] **Step 3: Write minimal implementation**

In `taskgate/cmd/ai.go`, add the import lines `"crypto/sha256"` and `"encoding/hex"` to the import block, then add the helper and update `snapshotDirFn`:

```go
func snapshotDirName(root string) string {
	sum := sha256.Sum256([]byte(root))
	return hex.EncodeToString(sum[:])[:12]
}
```

Change the final return of `snapshotDirFn` from:

```go
	return filepath.Join(home, ".taskgate", "snapshots", filepath.Base(root)), nil
```

to:

```go
	return filepath.Join(home, ".taskgate", "snapshots", snapshotDirName(root)), nil
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd taskgate && go test ./cmd/ -run TestSnapshot`
Expected: PASS (new test plus existing `TestSnapshotInstall_*`, which use the override and are unaffected)

- [ ] **Step 5: Commit**

```bash
git add taskgate/cmd/ai.go taskgate/cmd/snapshot_test.go
git commit -m "feat(snapshot): name snapshot dir by hash of project path"
```

---

### Task 2: Shared resolver + `snapshot path [path]`

**Files:**
- Modify: `taskgate/cmd/snapshot.go` (add `resolveSnapshotDir`, `newSnapshotPathCmd`, `snapshotPath`; register in `newSnapshotCmd`)
- Test: `taskgate/cmd/snapshot_test.go`

**Interfaces:**
- Consumes: `snapshotDirFn`, `snapshotDirOverride` (existing, `taskgate/cmd/ai.go`)
- Produces:
  - `resolveSnapshotDir(args []string) (string, error)` — workdir is `args[0]` when present, else `os.Getwd()`; applies `snapshotDirOverride` when set, else `snapshotDirFn`.
  - `newSnapshotPathCmd() *cobra.Command` — `snapshot path [path]`, `cobra.MaximumNArgs(1)`.

- [ ] **Step 1: Write the failing test**

Add to `taskgate/cmd/snapshot_test.go`:

```go
func TestSnapshotPath_PrintsResolvedDir(t *testing.T) {
	snapshotDirOverride = func(cwd string) (string, error) {
		return filepath.Join("/snap", filepath.Base(cwd)), nil
	}
	t.Cleanup(func() { snapshotDirOverride = nil })

	var out bytes.Buffer
	root := newRootCmd()
	root.SetOut(&out)
	root.SetArgs([]string{"snapshot", "path", "/Users/yn/work/taskgate"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := strings.TrimSpace(out.String())
	if got != "/snap/taskgate" {
		t.Errorf("expected /snap/taskgate, got %q", got)
	}
}
```

Add `"bytes"` and `"strings"` to the test file's import block if not already present.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd taskgate && go test ./cmd/ -run TestSnapshotPath`
Expected: FAIL — unknown command `"path"` for `snapshot` (non-nil error from Execute)

- [ ] **Step 3: Write minimal implementation**

In `taskgate/cmd/snapshot.go`, register the command inside `newSnapshotCmd` (after the `install` line):

```go
	snapshotCmd.AddCommand(newSnapshotPathCmd())
```

Add these functions to `taskgate/cmd/snapshot.go`:

```go
func resolveSnapshotDir(args []string) (string, error) {
	var workdir string
	if len(args) == 1 {
		workdir = args[0]
	} else {
		var err error
		workdir, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("cannot determine working directory: %w", err)
		}
	}

	dirFn := snapshotDirFn
	if snapshotDirOverride != nil {
		dirFn = snapshotDirOverride
	}
	return dirFn(workdir)
}

func newSnapshotPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "path [path]",
		Short:         "Print the snapshot directory for a project",
		Args:          cobra.MaximumNArgs(1),
		RunE:          snapshotPath,
		SilenceErrors: true,
	}
}

func snapshotPath(cmd *cobra.Command, args []string) error {
	dir, err := resolveSnapshotDir(args)
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), dir)
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd taskgate && go test ./cmd/ -run TestSnapshot`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add taskgate/cmd/snapshot.go taskgate/cmd/snapshot_test.go
git commit -m "feat(snapshot): add 'snapshot path' subcommand"
```

---

### Task 3: `snapshot delete [path]`

**Files:**
- Modify: `taskgate/cmd/snapshot.go` (add `newSnapshotDeleteCmd`, `snapshotDelete`; register in `newSnapshotCmd`)
- Test: `taskgate/cmd/snapshot_test.go`

**Interfaces:**
- Consumes: `resolveSnapshotDir(args []string) (string, error)` (Task 2)
- Produces: `newSnapshotDeleteCmd() *cobra.Command` — `snapshot delete [path]`, `cobra.MaximumNArgs(1)`.

- [ ] **Step 1: Write the failing tests**

Add to `taskgate/cmd/snapshot_test.go`:

```go
func TestSnapshotDelete_RemovesDirAndReportsCount(t *testing.T) {
	snapDir := t.TempDir()
	makeScript(t, snapDir, "deploy", "#!/bin/sh\necho deploy")
	makeScript(t, snapDir, "build", "#!/bin/sh\necho build")

	snapshotDirOverride = func(string) (string, error) { return snapDir, nil }
	t.Cleanup(func() { snapshotDirOverride = nil })

	var out bytes.Buffer
	root := newRootCmd()
	root.SetOut(&out)
	root.SetArgs([]string{"snapshot", "delete"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(snapDir); !os.IsNotExist(err) {
		t.Errorf("expected snapshot dir to be removed, stat err = %v", err)
	}
	if !strings.Contains(out.String(), "deleted 2 script(s)") {
		t.Errorf("expected count in output, got %q", out.String())
	}
}

func TestSnapshotDelete_NoopWhenAbsent(t *testing.T) {
	snapDir := filepath.Join(t.TempDir(), "missing")
	snapshotDirOverride = func(string) (string, error) { return snapDir, nil }
	t.Cleanup(func() { snapshotDirOverride = nil })

	var out bytes.Buffer
	root := newRootCmd()
	root.SetOut(&out)
	root.SetArgs([]string{"snapshot", "delete"})
	if err := root.Execute(); err != nil {
		t.Fatalf("expected no error for absent snapshot, got %v", err)
	}

	if !strings.Contains(out.String(), "no snapshot found") {
		t.Errorf("expected 'no snapshot found' message, got %q", out.String())
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd taskgate && go test ./cmd/ -run TestSnapshotDelete`
Expected: FAIL — unknown command `"delete"` for `snapshot`

- [ ] **Step 3: Write minimal implementation**

In `taskgate/cmd/snapshot.go`, register the command inside `newSnapshotCmd` (after the `path` line):

```go
	snapshotCmd.AddCommand(newSnapshotDeleteCmd())
```

Add these functions to `taskgate/cmd/snapshot.go`:

```go
func newSnapshotDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "delete [path]",
		Short:         "Delete a project's snapshot directory",
		Args:          cobra.MaximumNArgs(1),
		RunE:          snapshotDelete,
		SilenceErrors: true,
	}
}

func snapshotDelete(cmd *cobra.Command, args []string) error {
	dir, err := resolveSnapshotDir(args)
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(cmd.OutOrStdout(), "no snapshot found at %s\n", dir)
			return nil
		}
		return fmt.Errorf("cannot read snapshot directory: %w", err)
	}

	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			count++
		}
	}

	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("cannot delete snapshot directory: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "deleted %d script(s) from %s\n", count, dir)
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd taskgate && go test ./cmd/`
Expected: PASS (all cmd tests)

- [ ] **Step 5: Commit**

```bash
git add taskgate/cmd/snapshot.go taskgate/cmd/snapshot_test.go
git commit -m "feat(snapshot): add 'snapshot delete' subcommand"
```

---

## Self-Review

**Spec coverage:**
- Hashed naming (spec §1) → Task 1.
- `snapshot path [path]`, optional arg / cwd default, single-line absolute path (spec §2) → Task 2.
- `snapshot delete [path]`, optional arg / cwd, `os.RemoveAll`, count message / no-op message, exit 0 (spec §3) → Task 3.
- No migration (spec Non-Goals) → nothing added; existing install tests still pass via override.
- Testing notes (spec Testing) → covered by Task 1–3 tests using `snapshotDirOverride`.

**Placeholder scan:** None — all steps contain concrete code and commands.

**Type consistency:** `snapshotDirName(root string) string`, `resolveSnapshotDir(args []string) (string, error)`, and the `new*Cmd()` constructors are referenced consistently across tasks. `resolveSnapshotDir` is defined in Task 2 and consumed in Task 3.
