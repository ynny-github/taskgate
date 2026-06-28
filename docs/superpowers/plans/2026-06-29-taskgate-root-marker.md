# `.taskgate` Root Marker Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace git-based project-root detection with a filesystem walk that finds the nearest ancestor containing a `.taskgate` directory, and unify task resolution and snapshot storage on that root.

**Architecture:** `detectProjectRoot` walks upward from the working directory looking for a `.taskgate` directory; the directory that contains it is the project root. `run`, `ai run`, and `snapshot install` all resolve `.taskgate/...` paths against this root. Snapshot storage moves from `~/.taskgate/snapshots/` to an XDG state directory so the home directory never carries a `.taskgate` marker that the walk could mistake for a project root.

**Tech Stack:** Go, cobra, standard library (`os`, `path/filepath`), `go test`.

## Global Constraints

- Module/package: changes live in `taskgate/cmd` (package `cmd`).
- No new third-party dependencies; standard library only.
- The marker is the mere presence of a `.taskgate` **directory** (contents not inspected); the nearest ancestor wins.
- `detectProjectRoot` returns `""` when no marker is found anywhere up to the filesystem root. Its signature stays `func detectProjectRoot(cwd string) string`.
- Snapshot storage path: `<state>/taskgate/snapshots/<basename(root)>/`, where `<state>` is `$XDG_STATE_HOME` if set, else `~/.local/state`.
- Run each Go command from the repo root (`/Users/yn/.herdr/worktrees/taskgate/change-project-root-mark`) unless noted; package tests run with `go test ./taskgate/cmd/`.
- Commit messages follow Conventional Commits (`.claude/rules/git-commit.md`).

---

### Task 1: Replace git detection with a `.taskgate` filesystem walk

**Files:**
- Modify: `taskgate/cmd/run.go` (function `detectProjectRoot`, lines 57-63)
- Test: `taskgate/cmd/run_test.go` (add `TestDetectProjectRoot_*`)

**Interfaces:**
- Consumes: nothing new.
- Produces: `func detectProjectRoot(cwd string) string` — returns the nearest ancestor directory (starting at `cwd`) that contains a `.taskgate` directory, or `""` if none exists up to the filesystem root.

- [ ] **Step 1: Write the failing tests**

Add to `taskgate/cmd/run_test.go`:

```go
func TestDetectProjectRoot_FindsMarkerInParent(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".taskgate"), 0755); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(tmp, "a", "b")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	got := detectProjectRoot(sub)
	if got != tmp {
		t.Errorf("detectProjectRoot(%q) = %q, want %q", sub, got, tmp)
	}
}

func TestDetectProjectRoot_NoMarkerReturnsEmpty(t *testing.T) {
	tmp := t.TempDir()
	sub := filepath.Join(tmp, "a", "b")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	got := detectProjectRoot(sub)
	if got != "" {
		t.Errorf("detectProjectRoot(%q) = %q, want empty", sub, got)
	}
}

func TestDetectProjectRoot_NearestAncestorWins(t *testing.T) {
	tmp := t.TempDir()
	outer := filepath.Join(tmp, "outer")
	inner := filepath.Join(outer, "inner")
	if err := os.MkdirAll(filepath.Join(outer, ".taskgate"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(inner, ".taskgate"), 0755); err != nil {
		t.Fatal(err)
	}
	start := filepath.Join(inner, "x")
	if err := os.MkdirAll(start, 0755); err != nil {
		t.Fatal(err)
	}
	got := detectProjectRoot(start)
	if got != inner {
		t.Errorf("detectProjectRoot(%q) = %q, want %q", start, got, inner)
	}
}

func TestDetectProjectRoot_IgnoresMarkerFile(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, ".taskgate"), []byte("not a dir"), 0644); err != nil {
		t.Fatal(err)
	}
	got := detectProjectRoot(tmp)
	if got != "" {
		t.Errorf("detectProjectRoot(%q) = %q, want empty (.taskgate is a file)", tmp, got)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./taskgate/cmd/ -run TestDetectProjectRoot -v`
Expected: FAIL — the current git-based `detectProjectRoot` returns `""` (no git repo in the temp dirs), so `FindsMarkerInParent` and `NearestAncestorWins` fail with a mismatched path.

- [ ] **Step 3: Replace `detectProjectRoot`**

In `taskgate/cmd/run.go`, replace the existing function (lines 57-63):

```go
func detectProjectRoot(cwd string) string {
	out, err := exec.Command("git", "-C", cwd, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
```

with:

```go
func detectProjectRoot(cwd string) string {
	dir, err := filepath.Abs(cwd)
	if err != nil {
		return ""
	}
	for {
		marker := filepath.Join(dir, ".taskgate")
		if info, err := os.Stat(marker); err == nil && info.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
```

`run.go` still imports `os`, `os/exec` (used by `runTask`), `path/filepath`, `strings`, and `fmt`; all remain in use, so no import edits are needed.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./taskgate/cmd/ -run TestDetectProjectRoot -v`
Expected: PASS (all four).

- [ ] **Step 5: Commit**

```bash
git add taskgate/cmd/run.go taskgate/cmd/run_test.go
git commit -m "feat(cmd): detect project root via .taskgate marker

What: Replaces git rev-parse with an upward filesystem walk that returns
the nearest ancestor containing a .taskgate directory.
Why: Decouple project-root detection from git so taskgate works in
directories not under git control."
```

---

### Task 2: Resolve `run` tasks against the project root

**Files:**
- Modify: `taskgate/cmd/run.go` (`runTask` lines 25-55, `resolveHumanTask` lines 65-81)
- Test: `taskgate/cmd/run_test.go` (rework `TestRunCmd_SetsProjectRoot`, replace `TestRunCmd_NoProjectRoot_OutsideRepo`)

**Interfaces:**
- Consumes: `detectProjectRoot(cwd) string` (Task 1).
- Produces: `func resolveHumanTask(root, taskName string) (string, error)` — searches `root/.taskgate/{human,shared}/<taskName>`; same error strings as before.

- [ ] **Step 1: Update `resolveHumanTask` and `runTask` to use the root**

In `taskgate/cmd/run.go`, change `resolveHumanTask`'s first parameter from `cwd` to `root` (body unchanged otherwise):

```go
func resolveHumanTask(root, taskName string) (string, error) {
	for _, subdir := range []string{"human", "shared"} {
		path := filepath.Join(root, ".taskgate", subdir, taskName)
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("cannot access task %q: %w", taskName, err)
		}
		if info.Mode()&0111 == 0 {
			return "", fmt.Errorf("task %q is not executable", taskName)
		}
		return path, nil
	}
	return "", fmt.Errorf("task %q not found in .taskgate/human/ or .taskgate/shared/", taskName)
}
```

Replace `runTask` (lines 25-55) with a version that computes the root once and reuses it:

```go
func runTask(cmd *cobra.Command, args []string) error {
	taskName := args[0]
	scriptArgs := args[1:]

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot determine working directory: %w", err)
	}

	root := detectProjectRoot(cwd)

	taskPath, err := resolveHumanTask(root, taskName)
	if err != nil {
		return err
	}

	c := exec.Command(taskPath, scriptArgs...)
	c.Stdout = cmd.OutOrStdout()
	c.Stderr = cmd.ErrOrStderr()
	c.Stdin = os.Stdin

	if root != "" {
		env := make([]string, 0, len(os.Environ())+1)
		for _, e := range os.Environ() {
			if !strings.HasPrefix(e, "TASKGATE_PROJECT_ROOT=") {
				env = append(env, e)
			}
		}
		c.Env = append(env, "TASKGATE_PROJECT_ROOT="+root)
	}

	return c.Run()
}
```

Note: when `root == ""`, `resolveHumanTask("", taskName)` joins to a relative `.taskgate/...` path; since `detectProjectRoot` only returns `""` when no `.taskgate` exists anywhere up the tree (including `cwd`), the stat fails and the existing "task not found" error is returned.

- [ ] **Step 2: Update `TestRunCmd_SetsProjectRoot` (drop git)**

In `taskgate/cmd/run_test.go`, replace the body of `TestRunCmd_SetsProjectRoot` (lines 133-164) with:

```go
func TestRunCmd_SetsProjectRoot(t *testing.T) {
	tmp := t.TempDir()
	realTmp, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		t.Fatal(err)
	}
	makeHumanScript(t, realTmp, "print-root", "#!/bin/sh\necho \"$TASKGATE_PROJECT_ROOT\"")

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(realTmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	var buf bytes.Buffer
	root := newRootCmd()
	root.SetArgs([]string{"run", "print-root"})
	root.SetOut(&buf)
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if got != realTmp {
		t.Errorf("TASKGATE_PROJECT_ROOT = %q, want %q", got, realTmp)
	}
}
```

(`makeHumanScript` already creates `realTmp/.taskgate/human/`, which makes `realTmp` the project root. The `exec` import in `run_test.go` is still used by `TestRunCmd_PropagatesExitCode`.)

- [ ] **Step 3: Replace `TestRunCmd_NoProjectRoot_OutsideRepo` with subdir + no-marker tests**

In `taskgate/cmd/run_test.go`, delete `TestRunCmd_NoProjectRoot_OutsideRepo` (lines 166-192) and add:

```go
func TestRunCmd_ResolvesTaskFromSubdir(t *testing.T) {
	tmp := t.TempDir()
	realTmp, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		t.Fatal(err)
	}
	makeHumanScript(t, realTmp, "print-root", "#!/bin/sh\necho \"$TASKGATE_PROJECT_ROOT\"")
	sub := filepath.Join(realTmp, "nested", "dir")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(sub); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	var buf bytes.Buffer
	root := newRootCmd()
	root.SetArgs([]string{"run", "print-root"})
	root.SetOut(&buf)
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if got != realTmp {
		t.Errorf("TASKGATE_PROJECT_ROOT = %q, want %q (root, not cwd)", got, realTmp)
	}
}

func TestRunCmd_NoMarker_TaskNotFound(t *testing.T) {
	tmp := t.TempDir()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	root := newRootCmd()
	root.SetArgs([]string{"run", "ghost"})
	err = root.Execute()
	if err == nil {
		t.Fatal("expected error when no .taskgate marker exists, got nil")
	}
	want := `task "ghost" not found in .taskgate/human/ or .taskgate/shared/`
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}
```

- [ ] **Step 4: Run the package tests**

Run: `go test ./taskgate/cmd/ -run 'TestRunCmd|TestResolveHumanTask|TestDetectProjectRoot' -v`
Expected: PASS — including the unchanged `TestResolveHumanTask_*` (their `tmp` argument now serves as `root` and still contains `.taskgate`).

- [ ] **Step 5: Commit**

```bash
git add taskgate/cmd/run.go taskgate/cmd/run_test.go
git commit -m "feat(cmd): resolve run tasks against project root

What: runTask now computes the project root once and resolves
.taskgate tasks against it, so run works from any subdirectory.
Why: Unify task lookup with root detection; previously run only
inspected cwd/.taskgate."
```

---

### Task 3: Resolve `ai run` snapshot-freshness check against the root

**Files:**
- Modify: `taskgate/cmd/ai.go` (`runAITask` lines 61-105, `checkSnapshotFresh` lines 107-133)

**Interfaces:**
- Consumes: `detectProjectRoot(cwd) string` (Task 1).
- Produces: `func checkSnapshotFresh(root, taskName, snapshotPath string) error` — compares the snapshot against `root/.taskgate/{ai,shared}/<taskName>`; returns `nil` when `root == ""` or no source file exists.

- [ ] **Step 1: Update `runAITask` to compute the root once**

In `taskgate/cmd/ai.go`, edit `runAITask`. After the `cwd` block, add the root, pass it to `checkSnapshotFresh`, and reuse it for the env var. The relevant edits:

Insert after the snapshot-dir resolution (`taskPath, err := resolveAITask(...)` block), replace the `checkSnapshotFresh(cwd, ...)` call:

```go
	root := detectProjectRoot(cwd)

	if err := checkSnapshotFresh(root, taskName, taskPath); err != nil {
		fmt.Fprintln(cmd.ErrOrStderr(), err.Error())
		return err
	}
```

and replace the env block (lines 94-102) so it reuses `root` instead of calling `detectProjectRoot(cwd)` again:

```go
	if root != "" {
		env := make([]string, 0, len(os.Environ())+1)
		for _, e := range os.Environ() {
			if !strings.HasPrefix(e, "TASKGATE_PROJECT_ROOT=") {
				env = append(env, e)
			}
		}
		c.Env = append(env, "TASKGATE_PROJECT_ROOT="+root)
	}
```

- [ ] **Step 2: Update `checkSnapshotFresh` to take the root**

Replace the signature and add the empty-root guard:

```go
func checkSnapshotFresh(root, taskName, snapshotPath string) error {
	if root == "" {
		return nil
	}
	var sourcePath string
	for _, subdir := range []string{"ai", "shared"} {
		p := filepath.Join(root, ".taskgate", subdir, taskName)
		if _, err := os.Stat(p); err == nil {
			sourcePath = p
			break
		}
	}
	if sourcePath == "" {
		return nil
	}

	snapshotBytes, err := os.ReadFile(snapshotPath)
	if err != nil {
		return fmt.Errorf("cannot read snapshot for %q: %w", taskName, err)
	}
	sourceBytes, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("cannot read source for %q: %w", taskName, err)
	}

	if !bytes.Equal(snapshotBytes, sourceBytes) {
		return fmt.Errorf("snapshot for %q is out of date; ask a human to run 'taskgate snapshot install' to review and approve the changes", taskName)
	}
	return nil
}
```

- [ ] **Step 3: Run the `ai` tests to verify no regression**

Run: `go test ./taskgate/cmd/ -run TestAIRunCmd -v`
Expected: PASS — the stale/fresh tests run with `cwd` containing `.taskgate/ai`, so `root == cwd` and behavior is unchanged; the override-based tests with no `.taskgate` get `root == ""` and skip the freshness check (returns `nil`), matching prior behavior.

- [ ] **Step 4: Commit**

```bash
git add taskgate/cmd/ai.go
git commit -m "feat(cmd): check ai snapshot freshness against project root

What: runAITask resolves the .taskgate source for freshness comparison
against the detected root and reuses that root for TASKGATE_PROJECT_ROOT.
Why: Make ai run consistent with root-based task resolution."
```

---

### Task 4: Move snapshot storage to an XDG state directory

**Files:**
- Modify: `taskgate/cmd/ai.go` (`snapshotDirFn` lines 18-28)
- Test: `taskgate/cmd/ai_test.go` (add `TestSnapshotDirFn_*`)

**Interfaces:**
- Consumes: `detectProjectRoot(cwd) string` (Task 1).
- Produces: `func snapshotDirFn(cwd string) (string, error)` — returns `<state>/taskgate/snapshots/<basename(root)>`, where `<state>` is `$XDG_STATE_HOME` or `~/.local/state`; errors with `cannot determine project root: .taskgate directory not found` when no root.

- [ ] **Step 1: Write the failing tests**

Add to `taskgate/cmd/ai_test.go`:

```go
func TestSnapshotDirFn_XDGStateHome(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".taskgate"), 0755); err != nil {
		t.Fatal(err)
	}
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)

	got, err := snapshotDirFn(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(state, "taskgate", "snapshots", filepath.Base(tmp))
	if got != want {
		t.Errorf("snapshotDirFn = %q, want %q", got, want)
	}
}

func TestSnapshotDirFn_DefaultsToLocalState(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".taskgate"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_STATE_HOME", "")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	got, err := snapshotDirFn(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(home, ".local", "state", "taskgate", "snapshots", filepath.Base(tmp))
	if got != want {
		t.Errorf("snapshotDirFn = %q, want %q", got, want)
	}
}

func TestSnapshotDirFn_NoRoot(t *testing.T) {
	tmp := t.TempDir() // no .taskgate anywhere
	_, err := snapshotDirFn(tmp)
	if err == nil {
		t.Fatal("expected error when no .taskgate marker, got nil")
	}
	want := "cannot determine project root: .taskgate directory not found"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./taskgate/cmd/ -run TestSnapshotDirFn -v`
Expected: FAIL — current `snapshotDirFn` builds `~/.taskgate/snapshots/...` and errors with the old git-specific message.

- [ ] **Step 3: Rewrite `snapshotDirFn`**

In `taskgate/cmd/ai.go`, replace `snapshotDirFn` (lines 18-28):

```go
func snapshotDirFn(cwd string) (string, error) {
	root := detectProjectRoot(cwd)
	if root == "" {
		return "", fmt.Errorf("cannot determine project root: .taskgate directory not found")
	}
	stateHome := os.Getenv("XDG_STATE_HOME")
	if stateHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot determine home directory: %w", err)
		}
		stateHome = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(stateHome, "taskgate", "snapshots", filepath.Base(root)), nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./taskgate/cmd/ -run TestSnapshotDirFn -v`
Expected: PASS (all three).

- [ ] **Step 5: Commit**

```bash
git add taskgate/cmd/ai.go taskgate/cmd/ai_test.go
git commit -m "feat(cmd): store snapshots under XDG state dir

What: snapshotDirFn now writes to \$XDG_STATE_HOME (or ~/.local/state)
under taskgate/snapshots/<project>, instead of ~/.taskgate/snapshots.
Why: Keep ~/.taskgate from existing so the home directory is never
mistaken for a project root by the .taskgate marker walk."
```

---

### Task 5: Resolve `snapshot install` source scripts against the root

**Files:**
- Modify: `taskgate/cmd/snapshot.go` (`snapshotInstall` lines 33-85)

**Interfaces:**
- Consumes: `detectProjectRoot(cwd) string` (Task 1), `snapshotDirFn` / `snapshotDirOverride` (Task 4).
- Produces: no new exported interface; `snapshot install` reads `root/.taskgate/{ai,shared}/`.

- [ ] **Step 1: Point `taskgateDir` at the root**

In `taskgate/cmd/snapshot.go`, inside `snapshotInstall`, after the `dir, err := dirFn(cwd)` block, change the source directory from `cwd` to the detected root. Replace:

```go
	taskgateDir := filepath.Join(cwd, ".taskgate")
```

with:

```go
	root := detectProjectRoot(cwd)
	taskgateDir := filepath.Join(root, ".taskgate")
```

(When no marker exists, `dirFn(cwd)` — i.e. `snapshotDirFn` — already returns the "cannot determine project root" error earlier in the function, so `taskgateDir` is only built when a root exists. Tests that set `snapshotDirOverride` run with `cwd` containing `.taskgate`, so `root == cwd`.)

- [ ] **Step 2: Run the snapshot tests to verify no regression**

Run: `go test ./taskgate/cmd/ -run TestSnapshotInstall -v`
Expected: PASS — each test's `cwd` contains `.taskgate`, so `root == cwd` and the copied scripts resolve identically.

- [ ] **Step 3: Run the full package test suite**

Run: `go test ./taskgate/cmd/ -v`
Expected: PASS (all tests in package `cmd`).

- [ ] **Step 4: Commit**

```bash
git add taskgate/cmd/snapshot.go
git commit -m "feat(cmd): install snapshots from project-root .taskgate

What: snapshot install reads ai/ and shared/ scripts from the detected
project root's .taskgate, not cwd's.
Why: Allow snapshot install to run from any subdirectory, consistent
with root-based task resolution."
```

---

### Task 6: Full build and end-to-end verification

**Files:**
- No source changes (verification only).

- [ ] **Step 1: Build the binary**

Run: `go build ./...`
Expected: builds with no errors.

- [ ] **Step 2: Run the entire test suite**

Run: `go test ./...`
Expected: PASS across all packages, including `tests/e2e/show` (the `show` suite does not depend on root detection).

- [ ] **Step 3: Manual smoke test from a subdirectory**

```bash
mkdir -p tmp/demo/.taskgate/human tmp/demo/nested
printf '#!/bin/sh\necho "root=$TASKGATE_PROJECT_ROOT"\n' > tmp/demo/.taskgate/human/show-root
chmod +x tmp/demo/.taskgate/human/show-root
( cd tmp/demo/nested && go run ../../../taskgate run show-root )
```

Expected: prints `root=<absolute path to>/tmp/demo` (resolved from the nested subdirectory). Clean up with `rm -rf tmp/demo`.

- [ ] **Step 4: No commit needed** (verification only; nothing changed).

---

## Self-Review Notes

- **Spec coverage:** §1 detection → Task 1; §2 unify task resolution → Tasks 2 (run), 3 (ai), 5 (snapshot install); §3 env var → folded into Tasks 2 & 3; §4 XDG snapshot storage + error message → Task 4; §5 no-marker behavior → Task 2 (`TestRunCmd_NoMarker_TaskNotFound`) and Task 4 (`TestSnapshotDirFn_NoRoot`); §Testing → tests embedded per task; §Migration → no code (documented in spec).
- **Type consistency:** `detectProjectRoot(cwd) string`, `resolveHumanTask(root, taskName)`, `checkSnapshotFresh(root, taskName, snapshotPath)`, `snapshotDirFn(cwd)` used identically across tasks.
- **`os/exec` in `run.go`** stays imported (used by `runTask`); the spec's "drop if unused" does not apply.
