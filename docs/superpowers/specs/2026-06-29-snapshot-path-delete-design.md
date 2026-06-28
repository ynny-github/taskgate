# Snapshot Directory Hashing + `snapshot path` / `snapshot delete`

Date: 2026-06-29
Status: Approved (design)

## Background

A taskgate "snapshot" is an approved, static copy of the scripts in
`.taskgate/ai/` and `.taskgate/shared/`, frozen under the user's home
directory so AI-facing commands run only human-reviewed scripts.

The snapshot directory is resolved by a single function, `snapshotDirFn`
(`taskgate/cmd/ai.go:18-28`), which both `snapshot install` and `ai run`
use. Today it names the directory after the git repo basename:

```go
return filepath.Join(home, ".taskgate", "snapshots", filepath.Base(root)), nil
```

This collides when two different projects share a basename
(e.g. `~/work/taskgate` and `~/other/taskgate` both map to
`snapshots/taskgate/`).

## Goals

1. Make the snapshot directory name unique per project path.
2. Add a way to print where a project's snapshot is stored (the hashed
   name is not human-readable).
3. Add a way to delete a project's snapshot.

Both new commands accept an optional `path` argument and fall back to the
current working directory when it is omitted. `snapshot install` keeps its
existing behavior (cwd-only, no path argument).

## Non-Goals

- Migration of snapshots created under the old basename naming. After this
  change, old snapshot directories are orphaned; users regenerate them by
  re-running `snapshot install`. (Pre-release assumption — no migration.)
- Deleting individual scripts within a snapshot. `snapshot delete` removes
  the whole per-project snapshot directory.

## Design

### 1. Hashed directory naming

Change `snapshotDirFn` to name the directory after a hash of the absolute
project root path:

```go
sum := sha256.Sum256([]byte(root))
name := hex.EncodeToString(sum[:])[:12]
return filepath.Join(home, ".taskgate", "snapshots", name), nil
```

- `root` comes from `detectProjectRoot(cwd)` (git root, absolute path).
- The first 12 hex chars of SHA-256 are used as the directory name.
- This is the single point of change; `install` and `ai run` both pick it
  up automatically.

### 2. `taskgate snapshot path [path]`

- Arg: optional `path`. When omitted, use `os.Getwd()`.
- Resolve via `snapshotDirFn` (honoring `snapshotDirOverride` in tests).
- Print the resolved absolute snapshot directory path on a single line,
  regardless of whether the directory exists.
- Intended for shell use: `$(taskgate snapshot path)`.

### 3. `taskgate snapshot delete [path]`

- Arg: optional `path`. When omitted, use `os.Getwd()`.
- Resolve via `snapshotDirFn`.
- Remove the resolved directory with `os.RemoveAll` (no confirmation
  prompt).
- Output (both exit 0):
  - existed and removed: `deleted N script(s) from <dir>`
  - absent: `no snapshot found at <dir>`

  N is the count of regular files in the directory before removal.

### Path resolution detail

When a `path` argument is given, it is treated as a project directory and
passed where cwd would otherwise be used, so `detectProjectRoot` resolves
the git root from it. When omitted, `os.Getwd()` supplies the directory.
This mirrors how the existing commands resolve the working directory.

## Affected files

- `taskgate/cmd/ai.go` — `snapshotDirFn` naming change (add `crypto/sha256`,
  `encoding/hex` imports).
- `taskgate/cmd/snapshot.go` — register `newSnapshotPathCmd` and
  `newSnapshotDeleteCmd` under `newSnapshotCmd`; add their implementations
  (or split into a new file alongside, following existing patterns).

## Testing

- Reuse the `snapshotDirOverride` hook to bypass git-root/hash resolution
  and point at a temp directory.
- Hashed naming: same root yields a stable name; different roots yield
  different names.
- `snapshot path`: prints the resolved path with and without a `path` arg.
- `snapshot delete`: removes an existing snapshot and reports the count;
  no-op with the "no snapshot found" message when the directory is absent;
  both exit 0.
