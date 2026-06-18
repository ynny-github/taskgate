# AI Output Contract: `taskgate ai show` envelope JSON

**Feature**: Show Subcommand — Tasks and Directories with Descriptions
**Date**: 2026-06-16

Defines the wire format that `taskgate ai show` writes to stdout. One JSON document per invocation, terminated by a single trailing newline. Top-level `"kind"` discriminates the four shapes: `listing` | `task` | `directory` | `error`.

---

## Common rules

- Encoding: UTF-8.
- Whitespace: implementations SHOULD emit compact JSON (no inter-token whitespace beyond what JSON requires). Consumers MUST tolerate either compact or pretty-printed input.
- Terminator: exactly one `\n` after the closing brace.
- Field set: every record's required fields are listed below. **Additional fields are forward-compatible**: consumers MUST ignore unknown fields.
- `null` vs omission:
  - `summary` is **always present** on records that describe an entry; `null` when no summary was extracted (FR-009). Never omitted.
  - `body` is **omitted** from the document when the resolved entry has no body annotation; never `null`.
  - `entries` is **always present** on `listing` and `directory` shapes; an empty array (`[]`) when the merged level has zero entries. Never omitted, never `null`.

---

## Shape: `listing` (no-argument)

Emitted for `taskgate ai show` with no positional argument.

```json
{
  "kind": "listing",
  "audience": "ai",
  "entries": [
    {"path": ".taskgate/shared/deploy", "kind": "directory", "summary": "Promote a build to an environment."},
    {"path": ".taskgate/ai/build",       "kind": "task",      "summary": "Build the project for the current platform."},
    {"path": ".taskgate/shared/lint",    "kind": "task",      "summary": null}
  ]
}
```

Fields:

| Field | Type | Required | Notes |
|---|---|---|---|
| `kind` | string (`"listing"`) | yes | Discriminator. |
| `audience` | string (`"human"` | `"ai"`) | yes | Which audience filter was applied. (`"ai"` for `taskgate ai show`.) |
| `entries` | array of `ChildRecord` | yes | Sorted per FR-007. May be empty. |

Note: `taskgate ai show` always emits `audience: "ai"`. The same envelope schema is reusable internally for the human form's machine path if ever needed, hence the explicit `audience` field.

---

## Shape: `task` (file target)

Emitted for `taskgate ai show <task-name>` where `<task-name>` resolves to a task file.

```json
{
  "kind": "task",
  "path": ".taskgate/shared/lint",
  "summary": "Lint the codebase with project rules.",
  "body": "Runs project linters and exits non-zero on any error.\nReads CONFIG from the environment.",
  "audience": "ai"
}
```

When the task has no body:

```json
{
  "kind": "task",
  "path": ".taskgate/shared/lint",
  "summary": "Lint the codebase with project rules.",
  "audience": "ai"
}
```

When the task has no summary annotation but does have a body:

```json
{
  "kind": "task",
  "path": ".taskgate/shared/lint",
  "summary": null,
  "body": "…multi-line body…",
  "audience": "ai"
}
```

Fields:

| Field | Type | Required | Notes |
|---|---|---|---|
| `kind` | string (`"task"`) | yes | Discriminator. |
| `path` | string | yes | Real physical path under `.taskgate/`. |
| `summary` | string \| `null` | yes | `null` when no summary annotation found. |
| `body` | string | no | Verbatim string from the YAML `body` field (typically authored as a `body: |` literal-block, so embedded newlines are preserved). Omitted when no body annotation. |
| `audience` | string (`"human"` | `"ai"`) | yes | |

---

## Shape: `directory` (directory target)

Emitted for `taskgate ai show <directory-name>` where `<directory-name>` resolves to a directory.

```json
{
  "kind": "directory",
  "path": ".taskgate/shared/deploy",
  "summary": "Promote a build to an environment.",
  "body": "Each child task corresponds to a deploy target. Children run idempotently.",
  "audience": "ai",
  "entries": [
    {"path": ".taskgate/shared/deploy/canary",  "kind": "task",      "summary": "Promote to the canary fleet."},
    {"path": ".taskgate/shared/deploy/prod",    "kind": "task",      "summary": "Promote to production."}
  ]
}
```

When the directory has no `_index`, both `summary` and `body` are absent vs. present per the same rules: `summary` is `null`, `body` is omitted.

```json
{
  "kind": "directory",
  "path": ".taskgate/shared/deploy",
  "summary": null,
  "audience": "ai",
  "entries": [ … ]
}
```

Fields:

| Field | Type | Required | Notes |
|---|---|---|---|
| `kind` | string (`"directory"`) | yes | |
| `path` | string | yes | Real physical path. |
| `summary` | string \| `null` | yes | From `_index` if present; `null` otherwise. |
| `body` | string | no | From `_index` if present; omitted otherwise. |
| `audience` | string (`"human"` | `"ai"`) | yes | |
| `entries` | array of `ChildRecord` | yes | Immediate children only (FR-010); sorted per FR-007. May be empty. |

---

## Shape: `error`

Emitted on any non-success outcome. The error envelope replaces the entire payload; no partial listing is written alongside it.

### `error: "collision"` (exit 4)

```json
{
  "kind": "error",
  "error": "collision",
  "message": "name \"build\" collides across audience bucket and shared",
  "name": "build",
  "paths": [".taskgate/ai/build", ".taskgate/shared/build"]
}
```

| Field | Type | Required |
|---|---|---|
| `kind` | string (`"error"`) | yes |
| `error` | string (`"collision"`) | yes |
| `message` | string | yes — human-readable, may change between releases |
| `name` | string | yes — the colliding logical name |
| `paths` | array of string | yes — every conflicting real path; length ≥ 2 |

### `error: "not_found"` (exit 3)

```json
{
  "kind": "error",
  "error": "not_found",
  "message": "\"foo\" not found in .taskgate/ai or .taskgate/shared",
  "name": "foo",
  "searched": [".taskgate/ai", ".taskgate/shared"]
}
```

| Field | Type | Required |
|---|---|---|
| `kind` | string (`"error"`) | yes |
| `error` | string (`"not_found"`) | yes |
| `message` | string | yes |
| `name` | string | yes |
| `searched` | array of string | yes — bucket directories that were scanned |

### `error: "invalid_input"` (exit 2)

```json
{
  "kind": "error",
  "error": "invalid_input",
  "message": "taskgate ai show accepts run-style names, not filesystem paths",
  "input": ".taskgate/ai/build",
  "reason": "filesystem_path"
}
```

| Field | Type | Required |
|---|---|---|
| `kind` | string (`"error"`) | yes |
| `error` | string (`"invalid_input"`) | yes |
| `message` | string | yes |
| `input` | string | yes — verbatim user input |
| `reason` | string | yes — one of `"filesystem_path"` \| `"absolute_path"` \| `"parent_escape"` \| `"empty"` |

### `error: "workspace_missing"` (exit 5)

```json
{
  "kind": "error",
  "error": "workspace_missing",
  "message": ".taskgate/ not found at the current project root"
}
```

| Field | Type | Required |
|---|---|---|
| `kind` | string (`"error"`) | yes |
| `error` | string (`"workspace_missing"`) | yes |
| `message` | string | yes |

### `error: "io"` (exit 1)

Generic catch-all for I/O failures during read.

```json
{
  "kind": "error",
  "error": "io",
  "message": "permission denied reading .taskgate/shared/build",
  "path": ".taskgate/shared/build"
}
```

| Field | Type | Required |
|---|---|---|
| `kind` | string (`"error"`) | yes |
| `error` | string (`"io"`) | yes |
| `message` | string | yes |
| `path` | string | no — present when the failure is associated with a single file |

---

## `ChildRecord` (used inside `entries[]`)

```json
{"path": ".taskgate/shared/deploy/prod", "kind": "task", "summary": "Promote to production."}
```

| Field | Type | Required | Notes |
|---|---|---|---|
| `path` | string | yes | Real physical path. |
| `kind` | string (`"task"` | `"directory"`) | yes | |
| `summary` | string \| `null` | yes | `null` when no summary annotation found. |

A `ChildRecord` never carries `body`; never carries nested `entries`. FR-010 (no recursion) is enforced at this layer.

---

## Forward-compatibility notes

- Consumers MUST switch on `kind` (and on `error` inside an error envelope) and MUST ignore unknown fields and unknown discriminator values (treat as "this version of taskgate is newer than my parser; fail soft").
- Producers MUST NOT rename or remove the fields documented above within the v1 schema. New fields are additive.
- A future version that introduces a new top-level `kind` value will document it here; consumers that don't recognize it should fall through to a generic "unsupported envelope" error.
