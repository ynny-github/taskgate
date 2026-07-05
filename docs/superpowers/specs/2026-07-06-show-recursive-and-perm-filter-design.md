# Design: `show` recursive listing, directory-description removal, and executable-only filtering

**Date**: 2026-07-06
**Status**: Approved (design)
**Affects**: `taskgate show`, `taskgate ai show`

## Summary

This change reshapes the `show` subcommand along three axes:

1. **No-argument invocation lists the whole merged tree recursively**, instead of only the immediate root level.
2. **The directory-description feature (`_index`) is removed entirely.** Directories no longer carry a summary/body, and `_index` loses its reserved status — it becomes an ordinary file.
3. **Files without an execute bit are ignored** from both listing and name resolution, matching the fact that `taskgate run` treats task files as executable scripts.

`show <task>` (single-task detail) is unchanged.

## Motivation

- A single no-argument `show` should give the operator or agent a complete picture of the workspace, not just the top level, so browsing does not require walking directory-by-directory.
- The directory-description feature added surface area (a reserved filename, an optional annotation source, double-listing suppression) for marginal value. Removing it simplifies the model.
- `taskgate run` will only run executable task files. Listing non-executable files in `show` advertises tasks that cannot be run, so they should be hidden.

## Behavior

### Invocation matrix

| Invocation | Behavior |
|---|---|
| `show` (no argument) | Walk the merged view **recursively** and emit **every** entry, depth-first, ordered per level by the FR-007 rule (directories first, then tasks; basename case-sensitive ascending). |
| `show <dir>` | Emit **only the immediate children** (tasks and sub-directories) of the resolved directory. The directory itself carries **no** summary/body. |
| `show <task>` | Unchanged: emit the task's real path, summary, and body (body omitted when absent). |

The no-argument case is recursive; an explicit directory target is one level deep. This asymmetry is intentional: the no-argument case is a full browse, an explicit directory is a focused look.

### Directory-description removal

- The `_index` reserved filename and all of its special handling are removed. `_index` is scanned, listed, and resolvable like any other file.
- Directories never carry an annotation. A directory's summary/body is always absent.
- Directory rows are **path-only** (no summary column).
- `directoryEnvelope` drops its `summary` and `body` fields.
- Child records for a directory always report `summary: null` (directories have no annotation to source one from).

### Executable-only filtering

- Applies to **regular files only**. A regular file is included only when it has at least one execute bit set (`mode & 0111 != 0`). Directories are listed regardless of their execute bit.
- A non-executable file is excluded from listings **and** from name resolution. `show <name>` targeting a non-executable file resolves to **not found**, consistent with the file being invisible.
- Symlinks are judged by their **resolved target's** effective mode (the same file `taskgate run` would execute), after the existing symlink-escape check (FR-008) passes. A symlink whose target escapes `.taskgate/` keeps its current escape handling and is not evaluated for the execute bit.
- Filtering happens at scan time (`scanBucket` / `scanBucketSegment`), so excluded files are naturally absent from the recursive walk and from collision detection.

### Collision detection

Collision detection is unchanged in spirit and extended to the recursive walk: at **every level the walk visits**, a name appearing in both the audience bucket and the shared bucket is a collision. The first collision aborts the invocation with a non-zero exit and no partial output of the conflicting region (human form: stderr warning; AI form: `error` envelope, shape per ADR-0003).

## Output forms

### Human form (`taskgate show`)

- **No argument (recursive)**: an **indented tree**. Each row is `basename` (directories get a trailing `/`), followed by a tab and the summary for task rows (directories have no summary). Depth is expressed by indentation.
- **`show <dir>`**: the resolved directory's **real path** as a header line, then its immediate children as a **one-level indented tree** (basenames), matching the recursive tree's row style.
- **`show <task>`**: unchanged.

The human form collapses real physical paths to basenames for readability. The precise physical path of each entry is carried by the AI form, which is the source of truth for paths.

### AI form (`taskgate ai show`)

- **No argument (recursive)**: the existing `listingEnvelope`, with `entries` holding **every** entry flattened in depth-first order. Each record carries its full real `path`, `kind`, and `summary` (`null` for directories). Hierarchy is expressed by the paths themselves; the envelope stays flat.
- **`show <dir>`**: a directory record with an `entries` array of immediate children (full paths). The `summary`/`body` fields are removed from the directory envelope.
- **`show <task>`**: unchanged (`taskEnvelope`).

Example (no-argument AI form):

```json
{"kind":"listing","audience":"ai","entries":[
  {"path":".taskgate/shared/deploy","kind":"directory","summary":null},
  {"path":".taskgate/shared/deploy/prod","kind":"task","summary":"Production deploy"},
  {"path":".taskgate/shared/build","kind":"task","summary":"Build"}
]}
```

## Implementation notes

- `internal/show/mergedview.go`
  - Remove the `indexFilename` constant and every `_index` skip in `scanBucket` and `scanBucketSegment`.
  - Stop loading directory annotations: directories always resolve to an empty `AnnotationBlock`. Remove the directory branch of `loadAnnotationForWithNote` (the `_index` open).
  - Add the executable-only filter in `scanBucket` / `scanBucketSegment` for regular files (resolve symlinks for the mode check).
  - Add a recursive resolver (e.g. `ResolveTree`) that walks all levels, merging audience+shared per level, checking collisions per level, and returning a depth-first-ordered `[]Entry`. Add a `Depth int` field to `Entry` for tree indentation (zero for non-tree uses).
- `internal/show/show.go`
  - `runRoot` uses the recursive resolver instead of `ResolveRoot`.
- `internal/show/render_human.go`
  - Add the indented-tree renderer for the recursive listing.
  - `RenderHumanDirectory`: drop the summary/body section; render the path header plus a one-level indented child tree.
- `internal/show/render_ai.go`
  - Flatten the recursive walk into `listingEnvelope.entries` (depth-first).
  - Remove `summary`/`body` from `directoryEnvelope`.

## Documentation impact

- `docs/show/requirements.md`
  - **FR-003b**: reword — a directory target lists immediate children only, with **no** directory summary/body.
  - **FR-003c**: reword — the no-argument case presents the **recursive** merged tree (supersedes the immediate-children-only rule for this case).
  - **FR-004**: remove (directory description file).
  - **FR-010**: reword — the immediate-children boundary applies to an explicit directory target, not to the no-argument recursive case.
  - **FR-011**: remove (`_index` no longer double-listed because it is an ordinary file).
  - **FR-014 (new)**: regular files without an execute bit are excluded from listing and name resolution; a named non-executable file is "not found".
- `docs/show/glossary.md`: update the **Task entry** definition to "an executable file"; remove directory-description vocabulary.
- `docs/show/adr/0002-directory-description-filename.md`: mark **Superseded** (feature removed).
- New ADR: record the recursive no-argument listing and the executable-only filter.

## Out of scope

- Any change to `show <task>` detail output.
- Any change to `taskgate run` resolution.
- A recursive form for explicit directory targets (they stay one level deep).
