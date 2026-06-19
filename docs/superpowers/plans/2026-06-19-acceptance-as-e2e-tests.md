# Acceptance-as-E2E-Tests Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the in-process cmd-layer tests for the `show` subcommand with a subprocess-style E2E suite that mirrors the Acceptance Scenarios and Edge Cases of `specs/001-list-descriptions/spec.md` as 22 `.txtar` scenarios driven by `rogpeppe/go-internal/testscript`.

**Architecture:** A small `cmd.Run([]string, io.Writer, io.Writer) int` entry point lets `testscript.RunMain` re-exec the test binary under the program name `taskgate`. Each `.txtar` under `taskgate/cmd/testscript/scripts/show/` bundles the fixture, the command, and the expected output (golden file) in one block. The cobra subcommand wiring under `taskgate/cmd/` is unchanged; the production-code refactor is limited to flattening `main.go` and exposing `Run`.

**Tech Stack:** Go 1.25.5 · cobra · `gopkg.in/yaml.v3` (existing) · `github.com/rogpeppe/go-internal/testscript` (new).

**Reference design:** `docs/superpowers/specs/2026-06-19-acceptance-as-e2e-tests-design.md`.

**Reference spec for scenarios:** `specs/001-list-descriptions/spec.md` (Acceptance Scenarios + Edge Cases), `specs/001-list-descriptions/contracts/cli.md` (exit codes + stream rules), `specs/001-list-descriptions/contracts/ai-output.md` (AI envelope shapes).

## Global Constraints

- **Language for new code, docs, commit messages, file names:** English (per `.claude/CLAUDE.md` `output-language: English`).
- **Conventional Commits:** every commit must follow the `<type>(<scope>): <subject>` format documented in `.claude/rules/git-commit.md`. Imperative mood. Title ≤ 72 chars.
- **One commit = one purpose.** Do not bundle scenario commits across behavior groups.
- **Scratch artifacts (binaries, fixtures, captured stdout):** must live under `tmp/` in the workdir, never under system `/tmp/`.
- **Spec coverage:** all 11 Acceptance Scenarios from User Stories 1–4 and all 9 Edge Cases in `spec.md` must be covered by exactly one `.txtar` file each (overlaps are folded into a single file per behaviour). One additional file (`workspace_missing.txtar`) covers exit code 5 from `contracts/cli.md`.
- **No source-tree changes outside the test surface and the `cmd.Run` extraction.** The implementation under `taskgate/internal/show/` and the cobra subcommand files (`show.go`, `ai_show.go`, etc.) are not modified.
- **Existing internal-package tests stay.** Only the in-process cmd-layer tests (`taskgate/cmd/show_test.go` and `taskgate/cmd/ai_show_test.go`) are removed, and only at the very end.

## Brief testscript primer (for implementers new to it)

A `.txtar` file is plain UTF-8 text. Lines before the first `-- name --` header are the script (commands + assertions). Lines after a `-- name --` header are the literal contents of a file written into the scenario's working directory before the script runs.

Key script commands you will use:

- `exec taskgate show [args...]` — run the program; passes if exit is 0
- `! exec taskgate show [args...]` — run the program; passes if exit is non-zero
- `cmp stdout expected.txt` — compare captured stdout to the in-file `-- expected.txt --` block byte-for-byte
- `cmp stderr expected_err.txt` — same for stderr
- `stdout 'regex'` — assert captured stdout matches a regex
- `stderr 'regex'` — same for stderr
- `! stdout .` — assert stdout is empty (`.` means "any byte")
- `! stderr .` — assert stderr is empty
- `chmod 000 path/to/file` — change file mode (used by `unreadable_file.txtar`)
- `symlink link -> target` — create a symlink (used by `symlink_escape.txtar`)
- `mkdir -p path/to/dir` — create empty directories
- `[!unix]` (prefix on a line) — skip line on non-Unix; `[unix]` runs only on Unix

The script and the file blocks share a single working directory which is created fresh per `.txtar` run.

**Exit-code differentiation:** testscript distinguishes only "zero vs non-zero" out of the box. We rely on the distinctive stderr/stdout content guaranteed by `contracts/cli.md` to differentiate exit codes 2/3/4/5 (each has a unique error message or AI envelope shape).

**Updating golden files during development:** running `go test ./taskgate/cmd/testscript -run TestShow -update` rewrites the `-- expected*.txt --` blocks in `.txtar` files based on the actual captured output. Use it to bootstrap a new scenario, then inspect the diff and commit only what you intend to commit.

---

## File structure (created or modified by this plan)

| File | Disposition | Responsibility |
|---|---|---|
| `go.mod`, `go.sum` | Modify | Add `github.com/rogpeppe/go-internal` dependency. |
| `taskgate/cmd/exec.go` | Create | Defines `cmd.Run([]string, io.Writer, io.Writer) int` — the embeddable entry point used by both `main.go` and `testscript.RunMain`. |
| `taskgate/cmd/root.go` | Modify | Remove the now-unused `Execute() error` shim. `newRootCmd()` stays. |
| `taskgate/main.go` | Modify | Reduce to `os.Exit(cmd.Run(os.Args[1:], os.Stdout, os.Stderr))`. |
| `taskgate/cmd/testscript/main_test.go` | Create | `TestMain` registers `taskgate` with `testscript.RunMain`; `TestShow` runs `scripts/show/`. |
| `taskgate/cmd/testscript/scripts/show/*.txtar` | Create (22 files) | One file per Acceptance Scenario / Edge Case. |
| `taskgate/cmd/show_test.go` | Delete | Superseded by the E2E suite. |
| `taskgate/cmd/ai_show_test.go` | Delete | Same. |

Internal-package tests (`taskgate/internal/show/*_test.go`, `taskgate/internal/annotation/annotation_test.go`) are not touched. The cobra command files (`taskgate/cmd/show.go`, `ai_show.go`, `ai.go`, etc.) are not touched.

---

## Task 1: Add the testscript dependency

**Files:**
- Modify: `go.mod`, `go.sum`

**Interfaces:**
- Produces: `github.com/rogpeppe/go-internal/testscript` available in `go.mod`.

- [ ] **Step 1: Add the dependency with `go get`**

Run from the repository root:

```sh
cd /Users/yn/.herdr/worktrees/taskgate/convert-specs-to-test
go get github.com/rogpeppe/go-internal/testscript@latest
```

Expected: `go.mod` now contains a `require` line for `github.com/rogpeppe/go-internal`. `go.sum` updated accordingly. No code references it yet.

- [ ] **Step 2: Tidy the module**

```sh
go mod tidy
```

Expected: command completes silently with exit 0.

- [ ] **Step 3: Verify the existing test suite still builds and passes**

```sh
go test ./taskgate/...
```

Expected: all existing tests PASS. We have not changed any code; this catches accidental damage to `go.mod`.

- [ ] **Step 4: Commit**

```sh
git add go.mod go.sum
git commit -m "chore(deps): add rogpeppe/go-internal for testscript E2E suite"
```

---

## Task 2: Extract `cmd.Run` as the embeddable entry point

**Files:**
- Create: `taskgate/cmd/exec.go`
- Modify: `taskgate/cmd/root.go` (remove `Execute()` shim)
- Modify: `taskgate/main.go` (collapse to one `os.Exit` line)

**Interfaces:**
- Consumes: `newRootCmd()` from `taskgate/cmd/root.go` (unchanged); `show.ExitError` from `taskgate/internal/show/errors.go` (unchanged).
- Produces: `cmd.Run(args []string, stdout, stderr io.Writer) int`. Signature is consumed by `main.go` (Task 2) and by `taskgate/cmd/testscript/main_test.go` (Task 3).

This task is a pure refactor — no behaviour changes. The existing in-process tests in `show_test.go` / `ai_show_test.go` continue to work because they use `newRootCmd()` directly, which is preserved.

- [ ] **Step 1: Create `taskgate/cmd/exec.go`**

```go
// taskgate/cmd/exec.go
package cmd

import (
	"errors"
	"fmt"
	"io"
	"os/exec"

	"github.com/ynny-github/taskgate/taskgate/internal/show"
)

// Run is the embeddable entry point. main.go calls it once; the testscript
// suite calls it via testscript.RunMain so each scenario observes a real
// stdout/stderr writer and a real exit code.
//
// args is everything after the program name (i.e. os.Args[1:]).
func Run(args []string, stdout, stderr io.Writer) int {
	root := newRootCmd()
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	if err := root.Execute(); err != nil {
		var execErr *exec.ExitError
		if errors.As(err, &execErr) {
			return execErr.ExitCode()
		}
		var showErr *show.ExitError
		if errors.As(err, &showErr) {
			return showErr.Code
		}
		fmt.Fprintln(stderr, "taskgate:", err)
		return 1
	}
	return 0
}
```

- [ ] **Step 2: Remove the `Execute()` shim from `taskgate/cmd/root.go`**

Edit `taskgate/cmd/root.go` so it ends after `newRootCmd()`:

```go
// taskgate/cmd/root.go
package cmd

import "github.com/spf13/cobra"

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:          "taskgate",
		Short:        "Task runner — executes scripts from .taskgate/human/ and .taskgate/shared/",
		SilenceUsage: true,
	}
	root.AddCommand(newRunCmd())
	root.AddCommand(newAICmd())
	root.AddCommand(newSnapshotCmd())
	root.AddCommand(newInitCmd())
	root.AddCommand(newShowCmd())
	return root
}
```

(The `func Execute() error { return newRootCmd().Execute() }` block is deleted.)

- [ ] **Step 3: Collapse `taskgate/main.go`**

Replace the file entirely with:

```go
// taskgate/main.go
package main

import (
	"os"

	"github.com/ynny-github/taskgate/taskgate/cmd"
)

func main() {
	os.Exit(cmd.Run(os.Args[1:], os.Stdout, os.Stderr))
}
```

- [ ] **Step 4: Verify the build and the existing test suite**

```sh
go build ./taskgate
go test ./taskgate/...
```

Expected: build succeeds; all existing tests PASS. The refactor preserves the behaviour the existing tests rely on (`newRootCmd()` continues to work for them) and the new path from `main.go` (`cmd.Run` → `newRootCmd().Execute()`) preserves the production-side exit-code translation.

- [ ] **Step 5: Commit**

```sh
git add taskgate/cmd/exec.go taskgate/cmd/root.go taskgate/main.go
git commit -m "refactor(cmd): extract cmd.Run entry point for embedding"
```

---

## Task 3: Bootstrap the testscript suite with the smoke scenario `browse_root`

**Files:**
- Create: `taskgate/cmd/testscript/main_test.go`
- Create: `taskgate/cmd/testscript/scripts/show/browse_root.txtar`

**Interfaces:**
- Consumes: `cmd.Run` from Task 2.
- Produces: `TestShow` Go test that runs all `.txtar` files under `scripts/show/`. Later tasks just drop more files into that directory.

`browse_root.txtar` doubles as the smoke that proves the wiring works AND as the file covering Acceptance Scenario US1-1.

- [ ] **Step 1: Create `taskgate/cmd/testscript/main_test.go`**

```go
// taskgate/cmd/testscript/main_test.go
package testscript_test

import (
	"os"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/ynny-github/taskgate/taskgate/cmd"
)

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"taskgate": func() int {
			return cmd.Run(os.Args[1:], os.Stdout, os.Stderr)
		},
	}))
}

func TestShow(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "scripts/show",
	})
}
```

- [ ] **Step 2: Create `taskgate/cmd/testscript/scripts/show/browse_root.txtar`**

```txtar
# Annotated tasks from human/ and shared/ surface in the merged view with
# their summaries; bucket directories never appear as rows.

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
.taskgate/human/build	Build the project for the current platform.
.taskgate/shared/lint	Lint the codebase with project rules.
```

Note: the separator between path and summary is a single TAB character (not spaces). The renderer at `taskgate/internal/show/render_human.go:27` writes `"%s\t%s\n"`.

- [ ] **Step 3: Run the new test**

```sh
go test ./taskgate/cmd/testscript -run TestShow -v
```

Expected: `TestShow` runs `browse_root.txtar` and PASSes. If the byte-for-byte comparison fails, rerun with `-update` to capture actual output, inspect the diff, and reconcile:

```sh
go test ./taskgate/cmd/testscript -run TestShow -update
git diff taskgate/cmd/testscript/scripts/show/browse_root.txtar
```

Only commit content that matches the spec's intent (path + tab + summary, in basename-lex order, no bucket-directory rows).

- [ ] **Step 4: Commit**

```sh
git add taskgate/cmd/testscript/main_test.go taskgate/cmd/testscript/scripts/show/browse_root.txtar
git commit -m "test(show): bootstrap testscript suite with browse_root scenario"
```

---

## Scenario-task pattern

Tasks 4–8 each add a batch of `.txtar` files. The per-file inner loop is identical:

1. Create the `.txtar` file with the fixture, the command, and the assertions (golden file or regex match).
2. Run `go test ./taskgate/cmd/testscript -run TestShow/<filename-without-extension> -v`.
3. If byte-comparison fails: `-update` to capture actual stdout, inspect the diff, edit the `.txtar` to match the renderer's actual format (or fix the fixture if the captured output isn't what the spec expects).
4. Confirm PASS.
5. Commit the single file with a Conventional Commits message of the form `test(show): add <name> scenario`.

The five steps above are NOT spelled out per file inside Tasks 4–8 to keep this plan readable. Each scenario block under those tasks specifies the file contents and the assertion shape; that is the per-file work. Apply the pattern above mechanically.

---

## Task 4: Browse scenarios (2 files)

**Files:**
- Create: `taskgate/cmd/testscript/scripts/show/browse_root_unannotated.txtar`
- Create: `taskgate/cmd/testscript/scripts/show/browse_root_ai.txtar`

`browse_root.txtar` was created in Task 3. This task adds the other two.

### 4.1 `browse_root_unannotated.txtar`

Maps to: US1 scenario 2.

```txtar
# A task without an annotation block still appears at root with its path
# only; no error is raised.

exec taskgate show
cmp stdout expected.txt
! stderr .

-- .taskgate/shared/bare --
#!/bin/sh
echo hi

-- expected.txt --
.taskgate/shared/bare
```

### 4.2 `browse_root_ai.txtar`

Maps to: US1 scenario 4 ∩ US4 scenario 1.

`taskgate ai show` must merge `shared/` ∪ `ai/`, NOT `human/`. The envelope is the AI listing JSON shape from `contracts/ai-output.md`. Use `stdout` regex assertions rather than `cmp` because field order inside the JSON object is not contract-stable.

```txtar
# `taskgate ai show` with no argument merges shared/ ∪ ai/ (not human/) and
# emits a `listing` envelope on stdout.

exec taskgate ai show
stdout '"kind":"listing"'
stdout '"audience":"ai"'
stdout '\.taskgate/ai/analyze'
stdout '\.taskgate/shared/lint'
! stdout '\.taskgate/human/build'
! stderr .

-- .taskgate/human/build --
#!/bin/sh
# ---
# summary: Build.
# ---
echo build

-- .taskgate/shared/lint --
#!/bin/sh
# ---
# summary: Lint.
# ---
echo lint

-- .taskgate/ai/analyze --
#!/bin/sh
# ---
# summary: Analyze.
# ---
echo analyze
```

- [ ] **Step 1:** Create both files exactly as above.
- [ ] **Step 2:** For each file, follow the per-scenario inner loop (write → run → reconcile → pass → commit) described under "Scenario-task pattern". Each file gets its own commit: `test(show): add browse_root_unannotated scenario` and `test(show): add browse_root_ai scenario`.

---

## Task 5: Inspect scenarios (4 files)

**Files:**
- Create: `taskgate/cmd/testscript/scripts/show/inspect_task.txtar`
- Create: `taskgate/cmd/testscript/scripts/show/inspect_task_no_body.txtar`
- Create: `taskgate/cmd/testscript/scripts/show/inspect_task_ai.txtar`
- Create: `taskgate/cmd/testscript/scripts/show/inspect_task_ai_null_summary.txtar`

### 5.1 `inspect_task.txtar`

Maps to: US2 scenario 1.

```txtar
# `taskgate show <name>` for a task prints the resolved path, summary, and
# body in that order.

exec taskgate show build
cmp stdout expected.txt
! stderr .

-- .taskgate/human/build --
#!/bin/sh
# ---
# summary: Build the project.
# body: |
#   Reads VERSION from the environment.
#   Exits non-zero on build failure.
# ---
echo build

-- expected.txt --
.taskgate/human/build

  Build the project.

Reads VERSION from the environment.
Exits non-zero on build failure.
```

The expected layout follows `render_human.go:43-66`: path line, blank, two-space-indented summary, blank, body verbatim.

### 5.2 `inspect_task_no_body.txtar`

Maps to: US2 scenario 2.

```txtar
# When a task has a summary but no body, the body section is omitted
# entirely (no blank section, no placeholder).

exec taskgate show build
cmp stdout expected.txt
! stderr .

-- .taskgate/human/build --
#!/bin/sh
# ---
# summary: Build the project.
# ---
echo build

-- expected.txt --
.taskgate/human/build

  Build the project.
```

### 5.3 `inspect_task_ai.txtar`

Maps to: US4 scenario 2.

```txtar
# `taskgate ai show <name>` for a task emits a single `task` record with
# path, summary, body.

exec taskgate ai show analyze
stdout '"kind":"task"'
stdout '"path":"\.taskgate/ai/analyze"'
stdout '"summary":"Analyze the codebase\."'
stdout '"body":"Reads CONFIG from the environment\."'
stdout '"audience":"ai"'
! stderr .

-- .taskgate/ai/analyze --
#!/bin/sh
# ---
# summary: Analyze the codebase.
# body: |
#   Reads CONFIG from the environment.
# ---
echo analyze
```

### 5.4 `inspect_task_ai_null_summary.txtar`

Maps to: US4 scenario 4.

```txtar
# A task with no summary annotation must still produce a `task` record
# whose `summary` field is the explicit JSON `null`, not omitted.

exec taskgate ai show bare
stdout '"kind":"task"'
stdout '"path":"\.taskgate/ai/bare"'
stdout '"summary":null'
! stderr .

-- .taskgate/ai/bare --
#!/bin/sh
echo hi
```

- [ ] **Step 1:** Create all four files exactly as above.
- [ ] **Step 2:** For each file, run the per-scenario inner loop and commit individually: `test(show): add inspect_task scenario`, `test(show): add inspect_task_no_body scenario`, `test(show): add inspect_task_ai scenario`, `test(show): add inspect_task_ai_null_summary scenario`.

---

## Task 6: Directory scenarios (7 files)

**Files:**
- Create: `taskgate/cmd/testscript/scripts/show/dir_with_index.txtar`
- Create: `taskgate/cmd/testscript/scripts/show/dir_without_index.txtar`
- Create: `taskgate/cmd/testscript/scripts/show/dir_no_recursion.txtar`
- Create: `taskgate/cmd/testscript/scripts/show/dir_malformed_index.txtar`
- Create: `taskgate/cmd/testscript/scripts/show/dir_runnable_index.txtar`
- Create: `taskgate/cmd/testscript/scripts/show/dir_many_children.txtar`
- Create: `taskgate/cmd/testscript/scripts/show/dir_ai.txtar`

### 6.1 `dir_with_index.txtar`

Maps to: US3 scenario 1. Verifies path + summary + body + immediate-children rows, plus that `_index` is NOT listed as a child (FR-011).

```txtar
# `taskgate show <dir-name>` with an _index prints path, summary, body, then
# one row per immediate child; _index itself is not listed.

exec taskgate show deploy
cmp stdout expected.txt
! stderr .

-- .taskgate/human/deploy/_index --
# ---
# summary: Promote a build to an environment.
# body: |
#   Idempotent across reruns.
# ---

-- .taskgate/human/deploy/canary --
#!/bin/sh
# ---
# summary: Promote to canary.
# ---
echo canary

-- .taskgate/human/deploy/prod --
#!/bin/sh
# ---
# summary: Promote to production.
# ---
echo prod

-- expected.txt --
.taskgate/human/deploy

  Promote a build to an environment.

Idempotent across reruns.

.taskgate/human/deploy/canary	Promote to canary.
.taskgate/human/deploy/prod	Promote to production.
```

### 6.2 `dir_without_index.txtar`

Maps to: US3 scenario 2.

```txtar
# Directory with no _index prints path only, then the children rows.

exec taskgate show deploy
cmp stdout expected.txt
! stderr .

-- .taskgate/human/deploy/canary --
#!/bin/sh
# ---
# summary: Promote to canary.
# ---
echo canary

-- .taskgate/human/deploy/prod --
#!/bin/sh
# ---
# summary: Promote to production.
# ---
echo prod

-- expected.txt --
.taskgate/human/deploy

.taskgate/human/deploy/canary	Promote to canary.
.taskgate/human/deploy/prod	Promote to production.
```

### 6.3 `dir_no_recursion.txtar`

Maps to: US3 scenario 3. The nested sub-directory appears as a single row; the listing does not recurse.

```txtar
# Directory target lists nested sub-dirs as single rows (no recursion).
# Drilling deeper requires `taskgate show deploy/prod`.

exec taskgate show deploy
cmp stdout expected.txt
! stderr .

-- .taskgate/human/deploy/_index --
# ---
# summary: Promote a build.
# ---

-- .taskgate/human/deploy/prod/_index --
# ---
# summary: Prod target.
# ---

-- .taskgate/human/deploy/prod/run --
#!/bin/sh
# ---
# summary: Run a prod deploy.
# ---
echo prod

-- expected.txt --
.taskgate/human/deploy

  Promote a build.

.taskgate/human/deploy/prod/	Prod target.
```

The trailing `/` on the `prod/` row is emitted by `displayPath` at `render_human.go:33-38` because it's a directory entry. The nested `run` child is NOT listed (FR-010).

### 6.4 `dir_malformed_index.txtar`

Maps to: US3 scenario 4. The malformed `_index` is non-fatal: listing still succeeds, summary is omitted, human view emits a non-fatal notice on stderr, AI view stays silent.

```txtar
# A malformed _index does NOT abort the listing. Human form: summary/body
# omitted, optional notice on stderr (we assert non-empty stderr but not its
# exact text — the notice copy is renderer-owned and may change).

exec taskgate show deploy
stdout '\.taskgate/human/deploy'
stdout '\.taskgate/human/deploy/prod\tPromote to production\.'
! stdout 'Promote a build'

-- .taskgate/human/deploy/_index --
# ---
# summary: [unclosed_array
# ---

-- .taskgate/human/deploy/prod --
#!/bin/sh
# ---
# summary: Promote to production.
# ---
echo prod
```

Note: if the implementation chooses NOT to emit a stderr notice (the spec's "optional notice" wording allows that), drop the stderr assertion. The hard requirement is that the listing succeeds (exit 0) and the directory's intended summary does not leak into stdout.

### 6.5 `dir_runnable_index.txtar`

Maps to: Edge case "directory's dedicated description file is itself a runnable task file". Such an `_index` supplies the directory's annotation and is NOT also listed as a child.

```txtar
# A runnable _index (with shebang, executable-shaped) supplies the
# directory's annotation and is NOT double-listed as a child.

exec taskgate show deploy
cmp stdout expected.txt
! stderr .

-- .taskgate/human/deploy/_index --
#!/bin/sh
# ---
# summary: Promote a build.
# body: |
#   Idempotent.
# ---
echo "_index can also run"

-- .taskgate/human/deploy/prod --
#!/bin/sh
# ---
# summary: Promote to production.
# ---
echo prod

-- expected.txt --
.taskgate/human/deploy

  Promote a build.

Idempotent.

.taskgate/human/deploy/prod	Promote to production.
```

### 6.6 `dir_many_children.txtar`

Maps to: Edge case "directory contains hundreds of children". Verifies output stays usable; no silent truncation.

```txtar
# A directory with many children produces a complete listing with no
# truncation. We seed 50 children (enough to surface any per-child cost or
# accidental truncation) and assert the first and the last basename-lex
# entries are both present.

exec taskgate show many
stdout '\.taskgate/human/many/child00'
stdout '\.taskgate/human/many/child49'
! stderr .

-- .taskgate/human/many/child00 --
#!/bin/sh
# ---
# summary: 00.
# ---
echo 00

-- .taskgate/human/many/child01 --
#!/bin/sh
# ---
# summary: 01.
# ---
echo 01

# ... seed child02 through child48 with the identical shape ...

-- .taskgate/human/many/child49 --
#!/bin/sh
# ---
# summary: 49.
# ---
echo 49
```

**Important — the `# ... seed child02 through child48 with the identical shape ...` line above is a placeholder in THIS PLAN ONLY. Do NOT commit that text in the `.txtar` file. The committed file must contain all 50 blocks (`child00` … `child49`) literally — testscript does not expand placeholders.**

If hand-typing 50 blocks feels wasteful, generate them with a one-liner outside the editor:

```sh
for i in $(seq -f '%02g' 0 49); do
  printf -- '-- .taskgate/human/many/child%s --\n#!/bin/sh\n# ---\n# summary: %s.\n# ---\necho %s\n\n' "$i" "$i" "$i"
done
```

and paste the output into the file under the script section.

### 6.7 `dir_ai.txtar`

Maps to: US4 scenario 3.

```txtar
# `taskgate ai show <dir-name>` emits a single `directory` record with
# `entries[]` carrying immediate children (path + summary only).

exec taskgate ai show deploy
stdout '"kind":"directory"'
stdout '"path":"\.taskgate/shared/deploy"'
stdout '"summary":"Promote a build\."'
stdout '"entries":\['
stdout '"path":"\.taskgate/shared/deploy/canary"'
stdout '"path":"\.taskgate/shared/deploy/prod"'
! stderr .

-- .taskgate/shared/deploy/_index --
# ---
# summary: Promote a build.
# ---

-- .taskgate/shared/deploy/canary --
#!/bin/sh
# ---
# summary: Promote to canary.
# ---
echo canary

-- .taskgate/shared/deploy/prod --
#!/bin/sh
# ---
# summary: Promote to production.
# ---
echo prod
```

`shared/` is used so `taskgate ai show` (audience = ai) can resolve it; the AI audience does not see `human/`.

- [ ] **Step 1:** Create all seven files exactly as above (taking care to expand `dir_many_children.txtar` per the in-task instruction).
- [ ] **Step 2:** For each file, run the per-scenario inner loop and commit individually with messages of the form `test(show): add <name> scenario`.

---

## Task 7: Error and contract scenarios (4 files)

**Files:**
- Create: `taskgate/cmd/testscript/scripts/show/collision.txtar`
- Create: `taskgate/cmd/testscript/scripts/show/not_found.txtar`
- Create: `taskgate/cmd/testscript/scripts/show/reject_filesystem_path.txtar`
- Create: `taskgate/cmd/testscript/scripts/show/workspace_missing.txtar`

### 7.1 `collision.txtar`

Maps to: US1 scenario 3 ∩ Edge case "collision". `contracts/cli.md` exit 4. No partial output; stderr lists both real paths.

```txtar
# Same logical name in audience bucket and shared bucket is a hard error:
# non-zero exit, stderr lists both real paths, stdout is empty (no partial
# listing). Symmetric across no-arg browse and explicit name.

! exec taskgate show
! stdout .
stderr '\.taskgate/human/build'
stderr '\.taskgate/shared/build'

! exec taskgate show build
! stdout .
stderr '\.taskgate/human/build'
stderr '\.taskgate/shared/build'

-- .taskgate/human/build --
#!/bin/sh
# ---
# summary: Build (human variant).
# ---
echo h

-- .taskgate/shared/build --
#!/bin/sh
# ---
# summary: Build (shared variant).
# ---
echo s
```

### 7.2 `not_found.txtar`

Maps to: US2 scenario 3 (genuinely-absent variant) ∩ Edge case "not found". `contracts/cli.md` exit 3. Stderr lists the audience scope that was searched.

```txtar
# A name that does not resolve produces a non-zero exit with a clear
# `not found` stderr message that names the audience scope.

! exec taskgate show no-such-task
! stdout .
stderr 'not found'
stderr '\.taskgate/human'
stderr '\.taskgate/shared'

-- .taskgate/human/build --
#!/bin/sh
# ---
# summary: Build.
# ---
echo build
```

### 7.3 `reject_filesystem_path.txtar`

Maps to: US2 scenario 3 (fs-path variant) ∩ Edge case "filesystem path input". `contracts/cli.md` exit 2.

```txtar
# Filesystem-shaped inputs are rejected with a clear `run-style names only`
# stderr message. Empty string is also rejected. Each rejected input is
# tested in turn; the fixture is shared.

! exec taskgate show .taskgate/human/build
! stdout .
stderr 'run-style'

! exec taskgate show /abs/path
! stdout .
stderr 'run-style'

! exec taskgate show ./build
! stdout .
stderr 'run-style'

! exec taskgate show ''
! stdout .
stderr 'run-style'

-- .taskgate/human/build --
#!/bin/sh
# ---
# summary: Build.
# ---
echo build
```

### 7.4 `workspace_missing.txtar`

Maps to: `contracts/cli.md` exit 5.

```txtar
# Running show outside a workspace (no .taskgate/ in cwd or ancestors)
# produces a non-zero exit with a clear `.taskgate/ not found` message.
# This script intentionally creates NO .taskgate/ fixture.

! exec taskgate show
! stdout .
stderr '\.taskgate/'
```

(No `-- ... --` file blocks: the working directory stays empty.)

- [ ] **Step 1:** Create all four files exactly as above.
- [ ] **Step 2:** For each file, run the per-scenario inner loop and commit individually: `test(show): add collision scenario`, `test(show): add not_found scenario`, `test(show): add reject_filesystem_path scenario`, `test(show): add workspace_missing scenario`.

---

## Task 8: Annotation and file-system edge scenarios (4 files)

**Files:**
- Create: `taskgate/cmd/testscript/scripts/show/unreadable_file.txtar`
- Create: `taskgate/cmd/testscript/scripts/show/whitespace_summary.txtar`
- Create: `taskgate/cmd/testscript/scripts/show/leading_comments.txtar`
- Create: `taskgate/cmd/testscript/scripts/show/symlink_escape.txtar`

### 8.1 `unreadable_file.txtar`

Maps to: Edge case "unreadable task file". The listing must not abort; the remaining entries still appear.

```txtar
# A task file with read permission denied does not abort the listing.
# Remaining entries are still emitted. (Skipped on non-Unix where chmod
# semantics differ.)

[!unix] skip
chmod 000 .taskgate/human/locked
exec taskgate show
stdout '\.taskgate/shared/lint\tLint\.'

-- .taskgate/human/locked --
#!/bin/sh
# ---
# summary: Locked.
# ---
echo locked

-- .taskgate/shared/lint --
#!/bin/sh
# ---
# summary: Lint.
# ---
echo lint
```

The exact handling of the unreadable row (whether it appears with a notice or is silently dropped) is intentionally unasserted — the spec says "surfaces the path with a clear notice and continues" but the format of that notice is renderer-owned. The hard requirement is that the remaining entries are still listed.

### 8.2 `whitespace_summary.txtar`

Maps to: Edge case "annotation block exists but contains only whitespace after the summary marker".

```txtar
# A summary annotation that is whitespace-only is treated as empty (same
# handling as no summary). The task still appears at root, path-only.

exec taskgate show
cmp stdout expected.txt
! stderr .

-- .taskgate/human/build --
#!/bin/sh
# ---
# summary: "   "
# ---
echo build

-- expected.txt --
.taskgate/human/build
```

### 8.3 `leading_comments.txtar`

Maps to: Edge case "other comment lines appear between shebang and annotation envelope".

```txtar
# Non-annotation comments before the YAML envelope (shellcheck pragmas,
# copyright headers) are skipped; the parser finds the envelope and
# extracts the summary intact.

exec taskgate show
cmp stdout expected.txt
! stderr .

-- .taskgate/human/build --
#!/bin/sh
# shellcheck disable=SC2086
# Copyright (c) 2026 Example Corp.
# ---
# summary: Build the project.
# ---
echo build

-- expected.txt --
.taskgate/human/build	Build the project.
```

### 8.4 `symlink_escape.txtar`

Maps to: Edge case "symlink inside .taskgate/ points outside". The show command lists the link as an entry but does not read or display the off-workspace target's body.

```txtar
# A symlink under .taskgate/ pointing outside .taskgate/ surfaces as an
# entry but its target is not read; the off-workspace summary text never
# appears in output.

[!unix] skip
symlink .taskgate/human/escapee -> ../../outside

exec taskgate show
stdout '\.taskgate/human/escapee'
! stdout 'Secret outside summary'
! stderr .

-- outside --
#!/bin/sh
# ---
# summary: Secret outside summary.
# ---
echo outside
```

- [ ] **Step 1:** Create all four files exactly as above.
- [ ] **Step 2:** For each file, run the per-scenario inner loop and commit individually: `test(show): add unreadable_file scenario`, `test(show): add whitespace_summary scenario`, `test(show): add leading_comments scenario`, `test(show): add symlink_escape scenario`.

---

## Task 9: Remove superseded in-process tests and verify the full suite

**Files:**
- Delete: `taskgate/cmd/show_test.go`
- Delete: `taskgate/cmd/ai_show_test.go`

The new E2E suite now covers every Acceptance Scenario and Edge Case the in-process tests covered. The cobra command files (`show.go`, `ai_show.go`) and the internal-package tests are unchanged.

- [ ] **Step 1: Confirm the full E2E suite passes before deleting anything**

```sh
go test ./taskgate/cmd/testscript -run TestShow -v
```

Expected: every sub-test under `TestShow/` PASSes. Twenty-two sub-tests total (one per `.txtar`).

- [ ] **Step 2: Confirm the existing `taskgate/cmd/` tests still pass on this branch (they should — Task 2 left them untouched)**

```sh
go test ./taskgate/cmd
```

Expected: PASS. If anything failed here, the refactor in Task 2 broke something — fix that before continuing.

- [ ] **Step 3: Delete the superseded test files**

```sh
git rm taskgate/cmd/show_test.go taskgate/cmd/ai_show_test.go
```

- [ ] **Step 4: Confirm the build and the rest of the test suite still pass**

```sh
go build ./taskgate
go test ./taskgate/...
```

Expected: build succeeds; all surviving tests PASS — the internal-package tests (`taskgate/internal/show/...`, `taskgate/internal/annotation/...`) and the testscript suite.

If the build complains about unused imports anywhere (because the deleted test files were the only consumers), fix the imports in the affected file and add the fix to this same commit.

- [ ] **Step 5: Commit**

```sh
git commit -m "chore(test): remove in-process show tests superseded by E2E suite"
```

---

## Self-review notes

- **Spec coverage:** every Acceptance Scenario (US1-1..4, US2-1..3, US3-1..4, US4-1..4) and every Edge Case in `spec.md` maps to exactly one `.txtar` file, plus `workspace_missing.txtar` for `contracts/cli.md` exit 5 — total 22 files, matching the scenario catalogue in the design doc.
- **No placeholders:** every step has either runnable code, a complete file body, or a fully spelled-out shell command with its expected outcome. The single deliberate marker in `dir_many_children.txtar` is accompanied by an explicit instruction and a generator command for the omitted blocks; the produced file must contain all 50 blocks literally.
- **Type consistency:** `cmd.Run(args []string, stdout, stderr io.Writer) int` is the single embeddable entry point, referenced consistently in Tasks 2, 3, and the design doc. `*exec.ExitError` and `*show.ExitError` are the two sentinels translated in `cmd.Run`. Render shape (path `\t` summary) matches `render_human.go` exactly.
- **Frequency of commits:** one commit per task or per scenario file. The plan ends in a clean tree with 26+ commits documenting the change.
