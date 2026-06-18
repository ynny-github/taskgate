# Quickstart: Validating the Show Subcommand

**Feature**: Show Subcommand — Tasks and Directories with Descriptions
**Date**: 2026-06-16

Runnable validation scenarios that prove the feature works end-to-end. Each scenario maps back to one or more user stories and acceptance scenarios from `spec.md`. Refer to `contracts/cli.md` for exit codes and stream conventions; `contracts/ai-output.md` for the AI envelope shape.

---

## Prerequisites

- Go 1.25.5+ installed (matches `go.mod`).
- This repository checked out at `feature/show-index-subcommand`.
- All scratch artifacts live under the workdir's `tmp/` directory (already gitignored). Nothing is written to system `/tmp/` — keeps everything inside the workdir.

```sh
# Run this from the repository root.
cd /Users/yn/.herdr/worktrees/taskgate/feature-show-index-subcommand
WORKDIR="$(pwd)"

mkdir -p "$WORKDIR/tmp/bin" "$WORKDIR/tmp/scratch"
go build -o "$WORKDIR/tmp/bin/taskgate" ./taskgate
TASKGATE_BIN="$WORKDIR/tmp/bin/taskgate"

cd "$WORKDIR/tmp/scratch"
# No `git init` needed: show resolves `.taskgate/` from cwd, not from git.
```

`$TASKGATE_BIN` is used throughout. All scenarios run from `$WORKDIR/tmp/scratch`.

---

## Fixture

Drop the following structure into `$WORKDIR/tmp/scratch/`:

```text
.taskgate/
├── human/
│   ├── _index                 # describes the "human bucket" — IGNORED by show (audience-bucket _index has no effect)
│   ├── build                  # task: annotated
│   └── deploy/
│       ├── _index             # describes the deploy directory
│       ├── prod               # task: annotated
│       └── canary             # task: annotated
├── shared/
│   ├── lint                   # task: annotated
│   └── test                   # task: no annotation
└── ai/
    └── analyze                # task: annotated, only visible to `ai show`
```

Each task file is `chmod +x` and begins with `#!/bin/sh`. Annotations use the YAML front-matter format pinned in `research.md` R1:

```sh
#!/bin/sh
# ---
# summary: Build the project for the current platform.
# body: |
#   Reads VERSION from the environment.
#   Exits non-zero on build failure.
# ---
echo "build"
```

```sh
#!/bin/sh
# ---
# summary: Promote to production.
# ---
echo "deploy prod"
```

```sh
#!/bin/sh
echo "test"   # bare task: no annotation block
```

`_index` for `.taskgate/human/deploy/` (no shebang, not executable; the `# ` prefix is optional inside `_index`):

```text
# ---
# summary: Promote a build to an environment.
# body: |
#   Each child task corresponds to a deploy target. Children run idempotently.
# ---
```

---

## Scenario 1 — Root view (no argument), human form

Maps to **Story 1, scenarios 1, 2, 4**.

```sh
cd "$WORKDIR/tmp/scratch"
"$TASKGATE_BIN" show
```

**Expected**:

- Exit 0.
- stdout contains one row per merged-view entry of `shared/` ∪ `human/`:
  - `.taskgate/human/deploy/` (directory, with its summary)
  - `.taskgate/human/build` (task, with its summary)
  - `.taskgate/shared/lint` (task, with its summary)
  - `.taskgate/shared/test` (task, no summary — path-only row)
- stdout does **NOT** contain rows for `.taskgate/human/`, `.taskgate/shared/`, `.taskgate/ai/`, or `.taskgate/ai/analyze`.
- Ordering (FR-007): the directory row first, then task rows by basename: `build`, `lint`, `test`.
- stderr is empty.

---

## Scenario 2 — Root view (no argument), AI form

Maps to **Story 1 scenario 4** and **Story 4 scenario 1**.

```sh
"$TASKGATE_BIN" ai show
```

**Expected**:

- Exit 0.
- stdout is exactly one JSON document (per `contracts/ai-output.md`), of shape `{"kind":"listing","audience":"ai",…}`.
- `entries[]` contains `.taskgate/ai/analyze` and `.taskgate/shared/lint`, `.taskgate/shared/test`. Does **NOT** contain `.taskgate/human/*` entries.
- Each entry record has `path`, `kind`, `summary`. `.taskgate/shared/test` has `summary: null`.
- Sort order: directories before tasks (none here), then basename lex.

Pipe through `jq .` to inspect:

```sh
"$TASKGATE_BIN" ai show | jq .
```

---

## Scenario 3 — Single task target

Maps to **Story 2 scenarios 1, 2** and **Story 4 scenario 2**.

```sh
"$TASKGATE_BIN" show build
```

**Expected**:

- Exit 0.
- stdout prints the real path `.taskgate/human/build`, then the summary, then the body.

```sh
"$TASKGATE_BIN" show test
```

**Expected**:

- Exit 0.
- stdout prints the real path `.taskgate/shared/test`; no summary line; no body section.

AI form:

```sh
"$TASKGATE_BIN" ai show build | jq .
```

**Expected**: `{"kind":"task", "path":".taskgate/human/build", "summary":"…", "body":"…", "audience":"ai"}` — wait, this invocation is on the AI binary, which merges `shared`+`ai`, not `shared`+`human`. So `build` lives in `.taskgate/human/`, which the AI audience can't see. The actual expected outcome: an `error: "not_found"` envelope with exit 3. Verify by inspecting `kind` and exit code.

To test the AI form on a task the AI audience *can* see:

```sh
"$TASKGATE_BIN" ai show analyze | jq .
```

**Expected**: `{"kind":"task", "path":".taskgate/ai/analyze", "summary":"…", "body":"…", "audience":"ai"}`, exit 0.

---

## Scenario 4 — Directory target with `_index`

Maps to **Story 3 scenarios 1, 3** and **Story 4 scenario 3**.

```sh
"$TASKGATE_BIN" show deploy
```

**Expected**:

- Exit 0.
- stdout prints `.taskgate/human/deploy`, then the directory's summary ("Promote a build to an environment."), then the body, then a children listing showing `canary` and `prod` (sorted basename lex).
- `_index` is NOT among the listed children (FR-011).

```sh
"$TASKGATE_BIN" show deploy/prod
```

**Expected**: exit 0, shows `.taskgate/human/deploy/prod` as a single task target with its summary and body (verifying nested navigation, FR-003).

AI form:

```sh
"$TASKGATE_BIN" ai show deploy | jq .
```

Wait — same as Scenario 3: AI audience doesn't see `human/deploy/`, only `shared/` + `ai/`. So this returns `not_found`. To validate the directory-target AI shape, add a directory under `.taskgate/shared/` with its own `_index` and child tasks, then `ai show <that-dir-name>`.

---

## Scenario 5 — Directory without `_index`

Maps to **Story 3 scenario 2**.

Remove `.taskgate/human/deploy/_index` and rerun:

```sh
rm .taskgate/human/deploy/_index
"$TASKGATE_BIN" show deploy
```

**Expected**:

- Exit 0.
- stdout prints `.taskgate/human/deploy` only (no summary, no body), followed by the children listing as in Scenario 4. Restoring `_index` returns to Scenario 4 behavior.

---

## Scenario 6 — Name collision (hard error)

Maps to **Story 1 scenario 3**, edge case, and **FR-013**.

Create a colliding task:

```sh
cat > .taskgate/shared/build <<'EOF'
#!/bin/sh
# ---
# summary: Build (shared variant — created to verify collision detection).
# ---
exit 0
EOF
chmod +x .taskgate/shared/build
```

Now `build` exists in both `.taskgate/human/build` and `.taskgate/shared/build`.

```sh
"$TASKGATE_BIN" show
echo "exit: $?"
```

**Expected**:

- Exit **4**.
- stderr contains: `name "build" collides: .taskgate/human/build, .taskgate/shared/build` (or equivalent, listing both real paths).
- stdout is empty — no partial listing of the merged root.

```sh
"$TASKGATE_BIN" show build
echo "exit: $?"
```

**Expected**: same exit 4 and same collision warning. Explicit name reference also errors.

AI form:

```sh
"$TASKGATE_BIN" ai show; echo "exit: $?"
```

**Expected**:

- Exit 4.
- stdout contains a single JSON document `{"kind":"error","error":"collision","name":"build","paths":[".taskgate/human/build",".taskgate/shared/build"], "message":"…"}`.

Clean up the collision before continuing:

```sh
rm .taskgate/shared/build
```

---

## Scenario 7 — Invalid input (filesystem path rejection)

Maps to **Story 2 scenario 3** and **edge case** for filesystem-path input.

```sh
"$TASKGATE_BIN" show .taskgate/human/build; echo "exit: $?"
"$TASKGATE_BIN" show /abs/path/build;      echo "exit: $?"
"$TASKGATE_BIN" show ./build;               echo "exit: $?"
"$TASKGATE_BIN" show "";                    echo "exit: $?"
```

**Expected** (all four):

- Exit **2**.
- stderr contains a clear message indicating show accepts only `run`-style names.

AI form:

```sh
"$TASKGATE_BIN" ai show .taskgate/shared/lint | jq .
```

**Expected**: stdout is `{"kind":"error","error":"invalid_input","input":".taskgate/shared/lint","reason":"filesystem_path", …}`, exit 2.

---

## Scenario 8 — Name not found

Maps to **Story 2 scenario 3** (genuinely-absent variant).

```sh
"$TASKGATE_BIN" show no-such-task; echo "exit: $?"
```

**Expected**: exit **3**, stderr contains `"no-such-task" not found in .taskgate/human or .taskgate/shared`.

---

## Scenario 9 — Workspace missing

Maps to the contract's workspace-missing exit code.

```sh
cd "$WORKDIR/tmp"   # parent of scratch, no .taskgate/ here
"$TASKGATE_BIN" show; echo "exit: $?"
```

**Expected**: exit **5**, stderr: `.taskgate/ not found`.

---

## Scenario 10 — Legacy `list` is gone

Maps to research R4 (no backwards compatibility).

```sh
cd "$WORKDIR/tmp/scratch"
"$TASKGATE_BIN" list 2>&1; echo "exit: $?"
"$TASKGATE_BIN" ai list 2>&1; echo "exit: $?"
```

**Expected**: cobra's standard "unknown command" error, non-zero exit. No alias, no fallthrough.

---

## Scenario 11 — Non-`#` comment prefix (`//`, `--`, `;`)

Maps to research R1 supported prefixes. Confirms the recognizer auto-detects the prefix per file.

Add a JavaScript task and a Lua task to the fixture:

```sh
cat > .taskgate/human/devserver <<'EOF'
#!/usr/bin/env node
// ---
// summary: Run the dev server.
// body: |
//   Restarts on file changes.
// ---
require('./server').start();
EOF
chmod +x .taskgate/human/devserver

cat > .taskgate/shared/format <<'EOF'
#!/usr/bin/env lua
-- ---
-- summary: Format the project.
-- body: |
--   Skips vendor/.
-- ---
require('formatter').run()
EOF
chmod +x .taskgate/shared/format
```

Now run:

```sh
"$TASKGATE_BIN" show devserver
"$TASKGATE_BIN" show format
```

**Expected**:

- Both invocations exit 0.
- `show devserver` prints `.taskgate/human/devserver`, the summary `Run the dev server.`, and the body — extracted from the `// ---` envelope.
- `show format` prints `.taskgate/shared/format`, the summary `Format the project.`, and the body — extracted from the `-- ---` envelope.
- A root listing (`"$TASKGATE_BIN" show`) includes both rows alongside the sh-prefixed tasks, with all summaries intact regardless of the script's language.

AI form spot check:

```sh
"$TASKGATE_BIN" ai show format | jq '.summary, .body'
```

**Expected**: the summary and body strings, unaffected by which comment prefix the source file uses.

Clean up:

```sh
rm .taskgate/human/devserver .taskgate/shared/format
```

---

## Tearing down

```sh
rm -rf "$WORKDIR/tmp/scratch" "$TASKGATE_BIN"
```

---

## Coverage matrix

| Spec artifact | Covered by scenarios |
|---|---|
| US1: browse tasks at root | 1, 2 |
| US2: inspect single task | 3 |
| US3: browse a directory | 4, 5 |
| US4: AI consumption | 2, 3 (AI variant), 4 (AI variant) |
| Edge: collision | 6 |
| Edge: filesystem-path input | 7 |
| Edge: not-found | 8 |
| Edge: workspace missing | 9 |
| Non-`#` comment prefixes (`//`, `--`) | 11 |
| FR-007 sort order | 1, 4 |
| FR-010 no recursion | 4 (deploy + deploy/prod step) |
| FR-011 `_index` not double-listed | 4, 5 |
| FR-012 buckets invisible | 1, 2 |
| FR-013 collision = error | 6 |
| Legacy retirement | 10 |
