# CLI Contract: `taskgate show` and `taskgate ai show`

**Feature**: Show Subcommand — Tasks and Directories with Descriptions
**Date**: 2026-06-16

The user-facing surface for the show feature. This contract pins exit codes, argument parsing, streams, and the recognizable shape of both human-form and AI-form output.

---

## Command synopsis

```text
taskgate show [<name>]
taskgate ai show [<name>]
```

- `<name>` is an optional positional argument. When omitted, show emits the merged audience-filtered root view.
- No flags are introduced by this feature in v1.

### Removed in this feature

- `taskgate list` — removed; invoking it produces cobra's standard "unknown command" error.
- `taskgate ai list` — removed; same handling.

---

## Argument validation

Accepted forms for `<name>`:

- Bare basename, e.g. `build`, `lint`.
- Slash-separated nested name, e.g. `deploy/prod`, `release/canary/promote`.

Rejected forms (each maps to an `invalid_input` error; see exit codes below):

- Absolute path: any input whose first byte is `/`.
- Filesystem-style path: any input that contains the segment `.taskgate/` anywhere, or starts with `./` or `../`.
- Empty string (`""`): treated as `invalid_input:empty` rather than as "no name argument" — to keep `taskgate show ""` from silently behaving like `taskgate show`.

---

## Streams

- **stdout**: the entire successful payload (human-formatted text or the single AI envelope JSON document). A successful AI envelope ends with exactly one trailing `\n`.
- **stderr**: warnings and human-form error text (collisions, not-found, invalid-input). Stays empty on a successful invocation.
- **stdin**: not consumed; show never reads stdin.

The AI form (`taskgate ai show`) writes its **error envelope to stdout** as well (not stderr), so that AI consumers can `JSON.parse` a single stream regardless of outcome. Human-form errors go to stderr per Unix convention.

---

## Exit codes

| Code | Condition |
|---|---|
| `0` | Success — output rendered. |
| `1` | Generic failure not covered below (e.g., I/O error while reading a task file). |
| `2` | Invalid input — argument was a filesystem path, empty, or otherwise not a `run`-style name. |
| `3` | Not found — the supplied name did not resolve to any entry in the merged view. |
| `4` | Collision — same logical name present in both the audience bucket and the shared bucket; no partial output rendered. |
| `5` | Workspace error — `.taskgate/` not found or not accessible from the current working directory. |

These codes are stable and part of the contract; consumers may rely on the numeric value.

---

## Human form (`taskgate show`)

### Root view (no argument)

One row per merged-view entry, in FR-007 order (directories first, then tasks, basename lexicographic), formatted in two columns:

```text
<real path>    <summary or empty>
```

Concretely (illustrative; whitespace alignment is a layout choice owned by the renderer, not the contract):

```text
.taskgate/shared/deploy/    Promote a build to an environment.
.taskgate/human/build       Build the project for the current platform.
.taskgate/shared/lint       Lint the codebase with project rules.
```

Trailing `/` on directory paths is recommended but not required by the contract.

### Single task target (`taskgate show <task-name>`)

```text
<real path>

  <summary>

<body>
```

The body section is omitted entirely (no blank section, no "(no body)" placeholder) when no body annotation is present (FR-003a, FR-009).

### Directory target (`taskgate show <directory-name>`)

```text
<real path>

  <directory summary>

<directory body>

  <real child path>    <child summary or empty>
  …
```

When the directory has no `_index`, the summary and body sections are omitted; only the path and the children listing render.

### Error messages

- Collision (stderr, exit 4): `name "<name>" collides: <comma-separated real paths>`
- Not found (stderr, exit 3): `"<name>" not found in <bucket1> or <bucket2>`
- Invalid input (stderr, exit 2): `taskgate show accepts run-style names (bare or slash-separated), not filesystem paths`
- Workspace missing (stderr, exit 5): `.taskgate/ not found`

The renderer may add ANSI color when stdout/stderr is a TTY; consumers parsing the human form should expect uncolored bytes when stdout is piped.

---

## AI form (`taskgate ai show`)

The full schema and examples for every shape live in [`ai-output.md`](./ai-output.md). Summary of the contract:

- Exactly one JSON document on stdout per invocation, terminated by a single `\n`.
- Top-level `"kind"` field always present, one of `listing` | `task` | `directory` | `error`.
- Exit code follows the same table as the human form (`error.kind` maps onto the same numeric exit codes).

---

## Discoverability via `--help`

After this feature ships:

```text
$ taskgate --help
Usage:
  taskgate [command]

Available Commands:
  show       Show tasks and directories with summaries (merged shared+human view)
  ai         AI-facing taskgate commands
  run        Run a task from .taskgate/human/ or .taskgate/shared/
  snapshot   …
  init       …
```

```text
$ taskgate ai --help
Usage:
  taskgate ai [command]

Available Commands:
  show       Show tasks and directories with summaries (merged shared+ai view), in structured form
  run        Run an AI task from the snapshot directory
```

`taskgate list` and `taskgate ai list` no longer appear.

---

## Stability guarantees

- **Exit codes**: stable across this feature; downstream callers may switch on the numeric value.
- **AI envelope schema**: stable; additions are made by adding fields (never by renaming or removing). Schema evolution is out of scope for v1.
- **Human form layout**: not contract-stable. The "real path + summary" two-column shape is guaranteed; exact column widths, alignment, and color are renderer-owned and may change.
