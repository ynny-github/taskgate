# Design: Acceptance Scenarios as E2E Tests for the `show` Subcommand

**Date**: 2026-06-19
**Feature branch**: `convert-specs-to-test`
**Source spec**: `specs/001-list-descriptions/`

## Background

`specs/001-list-descriptions/spec.md` describes the `show` subcommand using prose: four User Stories with Given/When/Then Acceptance Scenarios (11 scenarios total) and an Edge Cases section (8 items). `specs/001-list-descriptions/quickstart.md` repeats the same surface as a runnable validation procedure that a human walks through with shell commands.

Today the project verifies these scenarios two ways: humans read the prose, and `taskgate/cmd/show_test.go` / `taskgate/cmd/ai_show_test.go` exercise the cobra command in-process via `root.Execute()`. The prose is the source of truth; the in-process tests are an incomplete shadow of it.

We want the **acceptance criteria themselves to live as executable tests**, so the spec's behavioural promises stop being prose-only and start being machine-checked at the same level a real user observes them — through a subprocess invocation of the built binary, observing real stdout, stderr, and exit codes.

## Scope

In scope:

- All Acceptance Scenarios from User Stories 1–4 of `spec.md` (11 scenarios).
- All items from the `Edge Cases` section of `spec.md` (9 items).
- Subprocess-style E2E execution of the `taskgate` binary.
- One scenario file is added from outside the explicit AS/Edge scope: `workspace_missing.txtar`, sourced from `contracts/cli.md` exit code 5. It is fundamental enough to a CLI's safety surface that omitting it would leave a gap a real operator notices in their first session.

Out of scope:

- Direct test coverage of every Functional Requirement (FR-001..FR-013). Most are already projected onto the Acceptance Scenarios; the rest stay as declarative requirements in the spec.
- Direct test coverage of Success Criteria (SC-001..SC-005). These are evaluation metrics meant for human review, not assertions.
- Deleting `spec.md` or `quickstart.md`. Both documents stay verbatim as the authored record.

## Approach

### Test runner: `rogpeppe/go-internal/testscript`

`testscript` is the de facto standard for CLI E2E in the Go ecosystem (used by `go`, `gopls`, `hugo`, and many others). Each `.txtar` file bundles three things into one block of text:

1. A `-- path --` filesystem layout (the fixture).
2. A short command script (`exec taskgate show`, etc.).
3. Inline expected output files compared with `cmp`.

This shape lets us write one Acceptance Scenario per file with Given (the `-- ... --` blocks), When (the `exec` line), and Then (the `cmp`/`stdout`/`stderr`/`!` assertions) all visible at once.

The binary is exercised as a real subprocess of the test process. `testscript.RunMain` hooks the program's entry point so the test binary can re-exec itself as `taskgate`, giving us real `os.Args`, real stdout/stderr file descriptors, and real exit codes — without paying the cost of `go build` per scenario.

### One subcommand, one directory

```
taskgate/cmd/testscript/
├── main_test.go
└── scripts/
    └── show/
        ├── browse_root.txtar
        ├── browse_root_unannotated.txtar
        ├── browse_root_ai.txtar
        ├── inspect_task.txtar
        ├── inspect_task_no_body.txtar
        ├── inspect_task_ai.txtar
        ├── inspect_task_ai_null_summary.txtar
        ├── dir_with_index.txtar
        ├── dir_without_index.txtar
        ├── dir_no_recursion.txtar
        ├── dir_malformed_index.txtar
        ├── dir_runnable_index.txtar
        ├── dir_many_children.txtar
        ├── dir_ai.txtar
        ├── collision.txtar
        ├── not_found.txtar
        ├── reject_filesystem_path.txtar
        ├── workspace_missing.txtar
        ├── unreadable_file.txtar
        ├── whitespace_summary.txtar
        ├── leading_comments.txtar
        └── symlink_escape.txtar
```

`scripts/show/` groups every scenario that belongs to the `show` subcommand. When a future subcommand (e.g. `run`) gets the same treatment, it lands in `scripts/run/` next to this one. No deeper grouping (no `browse/`, `task/`, `directory/`, `ai/` sub-folders) — every scenario is a peer of every other scenario, named for the behaviour it asserts.

### Naming

- File names describe the behaviour under test (`browse_root.txtar`, `collision.txtar`, `dir_with_index.txtar`). They do not encode spec labels like `us1`, `us2_3`, or `edge_5`. The scenario number on the spec side and the file name on the test side are intentionally independent — a scenario that splits or merges across documents stays grepable by behaviour either way.
- Sequence numbers (`01_`, `02_`) are not used. `testscript` runs files alphabetically but the scenarios are order-independent, so a stable execution order is not load-bearing.
- AI-variant scenarios use the `_ai` suffix on the base behaviour they vary (`browse_root_ai.txtar`, `inspect_task_ai.txtar`, `dir_ai.txtar`). Scenarios that exist only in the AI form get their own dedicated file (`inspect_task_ai_null_summary.txtar`).

### Header comments

Every `.txtar` opens with a single short comment line stating what it asserts. No `US-X` / `FR-XXX` references in the file — the file name carries the meaning, and the spec stays the canonical place where requirement IDs live.

```txtar
# Annotated tasks from both human/ and shared/ surface in the merged view with their summaries.

exec taskgate show
cmp stdout expected.txt
! stderr .

-- .taskgate/human/build --
#!/bin/sh
# ---
# summary: Build the project for the current platform.
# ---
echo build

-- .taskgate/shared/lint --
#!/bin/sh
# ---
# summary: Lint the codebase with project rules.
# ---
echo lint

-- expected.txt --
.taskgate/human/build    Build the project for the current platform.
.taskgate/shared/lint    Lint the codebase with project rules.
```

### Test functions

One `Test*` function per subcommand directory. `testscript.Run` does not recurse into sub-folders, so each subcommand gets its own entry point:

```go
func TestShow(t *testing.T) {
    testscript.Run(t, testscript.Params{Dir: "scripts/show"})
}
```

`go test ./taskgate/cmd/testscript -run TestShow` runs the whole `show` surface; `go test ./taskgate/cmd/testscript -run TestShow/dir_with_index` runs a single scenario.

### Binary entry point

`testscript.RunMain` requires a `func() int` keyed by program name. The current `taskgate/main.go` invokes `cmd.Execute()` which returns no value and uses `os.Exit` internally on failure. To plug into `RunMain` we extract a small, side-effect-explicit entry point:

```go
// taskgate/cmd/run.go (new function, alongside existing root setup)
func Run(args []string, stdout, stderr io.Writer) int {
    root := newRootCmd()
    root.SetArgs(args)
    root.SetOut(stdout)
    root.SetErr(stderr)
    if err := root.Execute(); err != nil {
        var exitErr *show.ExitError
        if errors.As(err, &exitErr) {
            return exitErr.Code
        }
        return 1
    }
    return 0
}
```

`taskgate/main.go` becomes a one-liner over `cmd.Run`. `taskgate/cmd/testscript/main_test.go` wires it in:

```go
func TestMain(m *testing.M) {
    os.Exit(testscript.RunMain(m, map[string]func() int{
        "taskgate": func() int {
            return cmd.Run(os.Args[1:], os.Stdout, os.Stderr)
        },
    }))
}
```

### Existing in-process tests

The acceptance-flavoured tests in `taskgate/cmd/show_test.go` and `taskgate/cmd/ai_show_test.go` are superseded by the new E2E suite. They are deleted to keep one source of truth per scenario. The internal-unit tests (`taskgate/internal/show/mergedview_test.go`, `taskgate/internal/show/render_test.go`, `taskgate/internal/annotation/annotation_test.go`) stay — they cover code paths below the CLI boundary that are not naturally observable through stdout/stderr.

Specifically, deleted in `taskgate/cmd/show_test.go`:

- `TestShow_RootView_Human`
- `TestShow_RootView_NoAnnotation_StillListed`
- `TestShow_RootView_Collision_Exit4`
- `TestShow_FileTarget_Human`
- `TestShow_InvalidInput_FilesystemPath_Exit2`
- `TestShow_InvalidInput_Empty_Exit2`
- `TestShow_NotFound_Exit3`
- `TestShow_DirectoryTarget_Human`
- `TestShow_DirectoryTarget_NoIndex_Human`
- `TestShow_NonShellPrefix`
- `TestShow_DescriptionFileIsolation`
- `TestShow_WorkspaceMissing_Exit5`
- `TestShow_LegacyListGone`
- `TestShow_RootView_AudienceAI_Skeleton`

And in `taskgate/cmd/ai_show_test.go`:

- `TestAIShow_RootView`
- `TestAIShow_TaskTarget`
- `TestAIShow_DirectoryTarget`
- `TestAIShow_Collision_Exit4_StdoutEnvelope`
- `TestAIShow_InvalidInput_Exit2_StdoutEnvelope`
- `TestAIShow_NotFound_Exit3_StdoutEnvelope`

The two test files themselves are removed once they contain no remaining tests.

### Documents kept verbatim

`specs/001-list-descriptions/spec.md` and `specs/001-list-descriptions/quickstart.md` are not edited as part of this change. The prose stays authoritative for "what we promised"; the new test suite is the executable mirror, not a replacement.

## Scenario catalogue

| File | Maps to (spec) | Asserts |
|---|---|---|
| `browse_root.txtar` | US1 sc.1 | Annotated tasks from `human/` and `shared/` surface in the merged view with summaries; bucket directories never appear as rows. |
| `browse_root_unannotated.txtar` | US1 sc.2 | Bare tasks (no annotation block) still appear with only their path. |
| `browse_root_ai.txtar` | US1 sc.4, US4 sc.1 | `taskgate ai show` merges `shared/` ∪ `ai/`, not `human/`; envelope is a `listing`. |
| `inspect_task.txtar` | US2 sc.1 | `taskgate show <name>` prints path, summary, body in that order. |
| `inspect_task_no_body.txtar` | US2 sc.2 | When body is absent the body section is omitted, not shown as empty. |
| `inspect_task_ai.txtar` | US4 sc.2 | `taskgate ai show <name>` returns a single `task` record with `path`, `summary`, `body`. |
| `inspect_task_ai_null_summary.txtar` | US4 sc.4 | Task with no summary emits `"summary": null` (not omitted) in the AI envelope. |
| `dir_with_index.txtar` | US3 sc.1 | Directory target with `_index` prints path + summary + body + immediate children. |
| `dir_without_index.txtar` | US3 sc.2 | Directory target without `_index` prints path + children only; summary/body sections omitted. |
| `dir_no_recursion.txtar` | US3 sc.3 | Nested sub-directory appears as a single row; recursion requires a follow-up `show deploy/prod`. |
| `dir_malformed_index.txtar` | US3 sc.4 | Malformed `_index` is non-fatal: listing still succeeds, summary omitted, human form emits a notice, AI form is silent. |
| `dir_runnable_index.txtar` | Edge case (runnable `_index`) | An executable `_index` supplies the directory's annotation and is not double-counted as a child. |
| `dir_many_children.txtar` | Edge case (many children) | Output stays usable when a directory has hundreds of children; no silent truncation. |
| `dir_ai.txtar` | US4 sc.3 | `taskgate ai show <dir>` returns a single `directory` record with `path`, `summary`, `body`, `entries[]` (children carry path + summary only). |
| `collision.txtar` | US1 sc.3, Edge case (collision) | Same name in audience bucket and shared bucket is a hard error: exit 4, no partial output, warning lists every conflicting real path. Symmetric across no-arg browse, explicit name, and directory children. |
| `not_found.txtar` | US2 sc.3 (genuinely-absent variant), Edge case (not found) | A name that does not resolve to any entry in the merged view exits 3 with a clear "not found" message listing the audience scope that was searched. |
| `reject_filesystem_path.txtar` | US2 sc.3 (fs-path variant), Edge case (fs path) | Any input shaped like a filesystem path (absolute, cwd-relative, or `.taskgate/`-prefixed) is rejected with exit 2 and a clear "run-style names only" message. Empty string is also rejected. |
| `workspace_missing.txtar` | `contracts/cli.md` exit 5 | `.taskgate/` absent from the current working tree exits 5 with a clear message. |
| `unreadable_file.txtar` | Edge case (unreadable task) | A task file with read denied surfaces with a notice; remaining entries still list. |
| `whitespace_summary.txtar` | Edge case (whitespace summary) | A summary marker followed by only whitespace is treated as empty, not malformed. |
| `leading_comments.txtar` | Edge case (leading comments) | Non-annotation comments between shebang and the annotation envelope (shellcheck pragmas, copyright headers) are skipped; only the YAML envelope is parsed. |
| `symlink_escape.txtar` | Edge case (symlink escape) | A symlink under `.taskgate/` whose target escapes `.taskgate/` is shown as an entry but its target is not read. |

Twenty-two files cover the eleven Acceptance Scenarios plus the nine Edge Cases plus the one contract-level workspace check. Overlaps between Acceptance Scenarios and Edge Cases (US1 sc.3 ≡ collision, US2 sc.3 ≡ fs-path & not-found) are folded into a single file per behaviour.

## Dependencies added

- `github.com/rogpeppe/go-internal/testscript` is added to `go.mod`. Single dependency, widely used, MIT-licensed, no transitive heavy surface.

## Trade-offs

- **Subprocess exec vs. in-process Execute.** Subprocess execution costs more wall-time per scenario (the test binary re-execs itself), but it observes the real exit code, the real stdout/stderr file descriptors, and any environmental coupling the in-process form hides. Since the user explicitly asked for the real E2E shape, we accept the wall-time cost.
- **No spec-ID labels in file names.** `us1_3` would make spec lookups easier but couples test names to a numbering scheme that changes when the spec is reorganised. We trade lookup ease for stability under spec edits; the file name still carries the behaviour.
- **`_ai` suffix vs. separate `ai/` folder.** A flat folder with an `_ai` suffix keeps related human/AI scenarios visually adjacent at the cost of mixed audiences in one directory. A separate folder would split closely-related assertions across folders. We picked adjacency.

## Out-of-scope follow-ups

- Migrating `taskgate run` and other subcommands to the same E2E style. Once `show` is converted, `scripts/run/` is a natural next step but is its own piece of work.
- CI integration tuning (parallelism, caching the test binary across scenarios). The default `go test` behaviour is sufficient at this surface size.
- A trace from test failure back to the originating Acceptance Scenario beyond the file name. If this matters later, a small mapping table can land in `spec.md` without changing any test.
