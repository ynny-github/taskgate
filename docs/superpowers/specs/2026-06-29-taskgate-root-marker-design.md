# Design: Mark the project root with a `.taskgate` directory

**Date:** 2026-06-29
**Status:** Approved (pending implementation)

## Summary

Today taskgate locates the project root by shelling out to git
(`git -C <cwd> rev-parse --show-toplevel`). This couples taskgate to git and
breaks in directories that are not git repositories.

This change replaces git-based detection with a filesystem walk: the project
root is the nearest ancestor directory (starting at the current working
directory) that contains a `.taskgate` directory. Task resolution and the
`TASKGATE_PROJECT_ROOT` environment variable are unified on this root, so
`run`, `ai run`, and `snapshot install` all behave consistently from any
subdirectory.

To avoid the home directory being mistaken for a project root, the snapshot
storage location moves out of `~/.taskgate` into an XDG state directory, so the
`.taskgate` name becomes a project marker exclusively.

## Motivation

- **Remove the git dependency.** taskgate should work in directories that are
  not under git control. git must no longer be a hard requirement.
- **Fix an existing inconsistency.** Root detection currently walks up (via
  git) while task lookup (`resolveHumanTask`, `checkSnapshotFresh`,
  `snapshotInstall`) reads `cwd/.taskgate/...` directly without walking up.
  Running from a subdirectory therefore fails to find tasks. Unifying both on
  the `.taskgate` marker resolves this.

## Current behavior (for reference)

- `detectProjectRoot(cwd)` runs `git rev-parse --show-toplevel`; returns `""`
  on any error.
- Used in three places:
  - `run.go` `runTask`: sets `TASKGATE_PROJECT_ROOT` for the executed task when
    the root is non-empty.
  - `ai.go` `runAITask`: same env handling.
  - `ai.go` `snapshotDirFn`: derives `~/.taskgate/snapshots/<basename(root)>`.
- Task lookup uses `cwd/.taskgate/...` directly (no walk-up):
  - `resolveHumanTask(cwd, name)` → `cwd/.taskgate/{human,shared}/<name>`
  - `checkSnapshotFresh(cwd, ...)` → `cwd/.taskgate/{ai,shared}/<name>`
  - `snapshotInstall` → `cwd/.taskgate/{ai,shared}/`

## Design

### 1. Root detection: `detectProjectRoot`

Replace the git call in `taskgate/cmd/run.go` with a filesystem walk:

- Resolve `cwd` to an absolute path, then walk upward one directory at a time.
- At each level, if `<dir>/.taskgate` exists **and is a directory**, return
  `<dir>`.
- If the filesystem root is reached without a match, return `""`.
- Remove the git invocation; drop the `os/exec` import from `run.go` if it
  becomes unused.

Signature is unchanged: `func detectProjectRoot(cwd string) string`.

The marker is the mere presence of a `.taskgate` **directory** (its contents
are not inspected). The "nearest ancestor" (deepest matching directory) wins.

### 2. Unify task resolution on the project root

Route every `.taskgate` lookup through the detected root instead of `cwd`:

- **`resolveHumanTask`** (`run.go`): take `root` instead of `cwd` and search
  `root/.taskgate/{human,shared}/<task>`. In `runTask`, call
  `detectProjectRoot(cwd)` first. When `root == ""`, return the existing
  not-found error (`task "<name>" not found in .taskgate/human/ or
  .taskgate/shared/`) — if no `.taskgate` exists anywhere up the tree, the task
  cannot exist.
- **`checkSnapshotFresh`** (`ai.go`): compare against
  `root/.taskgate/{ai,shared}/<task>` instead of `cwd/.taskgate/...`.
- **`snapshotInstall`** (`snapshot.go`): build `taskgateDir` from
  `root/.taskgate` instead of `cwd/.taskgate`. Obtain `root` via
  `detectProjectRoot(cwd)` directly (a separate concern from the snapshot
  storage path computed by `snapshotDirFn`).

Result: `run`, `ai run`, and `snapshot install` all resolve tasks against the
project root, so they work from any subdirectory.

### 3. `TASKGATE_PROJECT_ROOT` environment variable

Unchanged logic. `run.go` / `ai.go` continue to set `TASKGATE_PROJECT_ROOT`
for the executed task only when `detectProjectRoot(cwd)` returns a non-empty
value. Only the detection mechanism changed (git → `.taskgate` walk).

### 4. Snapshot storage location: `snapshotDirFn`

Move snapshot storage out of `~/.taskgate` into an XDG state directory so the
home directory no longer contains a `.taskgate` that the walk-up could mistake
for a project root.

- Use `$XDG_STATE_HOME` when set; otherwise default to `~/.local/state`.
- Storage path: `<state>/taskgate/snapshots/<basename(root)>/`.
- Update the no-root error message from the git-specific wording to:
  `cannot determine project root: .taskgate directory not found`.

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

### 5. Behavior when no marker is found

- `run`: returns the existing "task not found" error;
  `TASKGATE_PROJECT_ROOT` stays unset. No special "not a taskgate project"
  error is introduced.
- `ai run` / `snapshot install`: error via `snapshotDirFn` with
  `cannot determine project root: .taskgate directory not found`.

## Migration

Existing snapshots under `~/.taskgate/snapshots/` are not migrated
automatically. Snapshots are reproducible: running `taskgate snapshot install`
once recreates them under the new XDG location. Automatic migration is out of
scope (YAGNI).

## Testing

- **`TestRunCmd_SetsProjectRoot`** (`run_test.go`): drop `git init`; rely on the
  `tmp/.taskgate/human/` created by `makeHumanScript`. `tmp` becomes the root,
  so assert `TASKGATE_PROJECT_ROOT == tmp`. Keep `EvalSymlinks` for the
  macOS `/var` → `/private/var` realpath resolution.
- **`TestRunCmd_NoProjectRoot_OutsideRepo`**: its premise no longer holds (a
  `cwd/.taskgate` now makes cwd the root). Reshape into a subdirectory test:
  create `tmp/.taskgate/human/print-root`, run from `tmp/sub/`, and assert
  `TASKGATE_PROJECT_ROOT == tmp` (the root, not cwd) and that the task resolves
  against the root. Add a separate test for the "no `.taskgate` anywhere" case
  that asserts the "task not found" error.
- **`TestResolveHumanTask_*`**: follow the signature change to `root`. The
  existing cases create `.taskgate` directly under `tmp`, so `tmp` is the root
  and they still pass.
- **New `TestDetectProjectRoot_*`**: directly verify (a) discovery of a
  `.taskgate` in a parent from a nested subdirectory, (b) `""` when no
  `.taskgate` exists up to the filesystem root, and (c) selection of the
  nearest ancestor when multiple levels contain `.taskgate`.
- **Snapshot tests**: isolate the storage path with
  `t.Setenv("XDG_STATE_HOME", tmp)` and assert the new
  `<state>/taskgate/snapshots/<project>/` layout.
- **e2e** (`tests/e2e/...`): the `show` suite does not depend on root
  detection; confirm no changes are required.

## Out of scope

- Strict marker validation (requiring `.taskgate/{human,shared,ai}`): the
  storage relocation already prevents the home-directory false positive, so the
  simple "any `.taskgate` directory" marker is sufficient.
- Automatic snapshot migration from the old `~/.taskgate` location.
- A `GIT_CEILING_DIRECTORIES`-style ceiling on the walk-up.
