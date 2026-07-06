# Design: `show` / `ai show` Output Formatting

Date: 2026-07-07
Status: Approved (pending implementation plan)

## Problem

Two adjustments to the output of `taskgate show` and `taskgate ai show`:

1. **`show` (human form)** currently separates each entry name from its
   summary with a single tab (`render_human.go` `writeTreeRow`). Tab stops
   depend on name length, so summaries land at ragged columns and the listing
   is hard to scan:

   ```
   build	Build ESP32 apps (transmitter, receiver, or all).
   initialize	One-time project setup (submodules, hooks, deps, sandbox, snapshot).
   log_udp	Listen for UDP log messages and print them to stdout.
   ```

2. **`ai show` (AI form)** reports each entry's physical `path` but not the
   `run`-style name a caller passes to `taskgate run` / `taskgate ai run` /
   `taskgate ai show`. Callers must reconstruct it from the path.

## Scope

Both changes concern show output only. No change to resolution, collision
detection, audience merging, executable filtering, or exit codes.

## Design

### 1. `show` (human form): aligned summary column

Replace the tab separator with a padded, globally-aligned summary column.

- **Root (no argument, recursive tree)** — unchanged structure: directories
  render as `<indent><name>/`; tasks render as `<indent><name><pad><summary>`.
  Indentation stays two spaces per depth level.
- **Column position** — one column for the whole tree. Its start is
  `max(indentWidth + len(name))` over all **task rows that carry a non-empty
  summary**, plus a **two-space gap**. Directory rows and summary-less task
  rows do not participate in the width calculation and are printed as name
  only (no trailing padding).
- **No truncation** — long summaries are left to terminal wrapping. (The `...`
  in mockups during brainstorming was display-only.)
- **`show <dir>` (directory target)** — unchanged structure: real path, blank
  line, then immediate children. Children use the same aligned-column logic,
  with the column width computed within that one listing.
- **`show <task>` (task target)** — unchanged (path, blank line, indented
  summary, blank line, body).

Example root output:

```
deploy/
  prod      Promote to production.
  stg       Promote to staging.
build       Build ESP32 apps.
initialize  One-time project setup.
```

`prod`/`stg` (indent 2 + name) and root-level `build`/`initialize` all align
to the same summary column.

### 2. `ai show` (AI form): add `name`, keep everything else

The only change is a new `name` field carrying the `run`-style invocation
name. Structure, `kind` discriminators, directory entries, the recursive root
listing, and the non-recursive directory target are all **kept as-is**.

- **`name`** = the `run`-style name: the entry's `path` with its
  `.taskgate/<bucket>/` prefix removed (`bucket` ∈ `{human, ai, shared}`).
  Examples: `.taskgate/human/build` → `build`;
  `.taskgate/shared/deploy/prod` → `deploy/prod`;
  `.taskgate/shared/deploy` (directory) → `deploy`.
- Added to every child record, to the `task` envelope, and to the `directory`
  envelope's own identity.
- The listing key stays **`entries`** (records still include directories, so
  `commands` would misname them).

**Listing envelope (root, recursive):**

```json
{"kind":"listing","audience":"ai","entries":[
  {"name":"build","path":".taskgate/human/build","kind":"task","summary":"Build."},
  {"name":"deploy","path":".taskgate/shared/deploy","kind":"directory","summary":null},
  {"name":"deploy/prod","path":".taskgate/shared/deploy/prod","kind":"task","summary":"Prod."},
  {"name":"deploy/stg","path":".taskgate/shared/deploy/stg","kind":"task","summary":"Stg."}
]}
```

**Directory envelope (`ai show deploy`, non-recursive — immediate children):**

```json
{"kind":"directory","name":"deploy","path":".taskgate/shared/deploy","audience":"ai","entries":[
  {"name":"deploy/prod","path":".taskgate/shared/deploy/prod","kind":"task","summary":"Prod."},
  {"name":"deploy/stg","path":".taskgate/shared/deploy/stg","kind":"task","summary":"Stg."}
]}
```

**Task envelope (`ai show build`):**

```json
{"kind":"task","name":"build","path":".taskgate/human/build","summary":"Build.","body":"...","audience":"ai"}
```

- `summary` remains nullable (`null` when absent); `body` remains
  `omitempty`.
- `error` envelopes are unchanged.
- Ordering, recursion depth, collision detection, and audience merging are
  unchanged.

## Affected code

- `internal/show/render_human.go` — `writeTreeRow` (and its callers
  `RenderHumanTree`, `RenderHumanDirectory`) switch from tab separation to a
  computed, aligned column. Width computation is a pre-pass over the rows.
- `internal/show/render_ai.go` — add `Name` to `childRecord`, `taskEnvelope`,
  `directoryEnvelope`; populate it in `childRecords` and `renderAITarget`.
- A small helper to derive the `run`-style name from a physical `path`
  (strip `.taskgate/<bucket>/`).

## Affected docs & tests

- **ADR-0003** (AI wire format) — document the new `name` field on records
  and the task/directory envelopes.
- **requirements.md** — note the `name` field in the AI-form requirements
  (FR-005/FR-006 area); no behavioral FR changes.
- **glossary.md** — "Output record" gains the `run`-style `name` for the AI
  form.
- **e2e golden files** (`tests/e2e/show/testdata/golden/`) — regenerate for
  the aligned human column and the `name` field in AI JSON.
- **Unit tests** (`internal/show/render_test.go`) — cover column alignment
  (including nested rows and summary-less rows) and the `name` derivation.

## Out of scope

- No change to `run` / `ai run`.
- No truncation or width-clamping of summaries.
- No change to collision, symlink, or executable-filter behavior.
