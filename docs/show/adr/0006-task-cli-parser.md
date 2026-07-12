# ADR-0006: Task CLI parser (`args`/`flags` declaration)

**Status**: Accepted (2026-07-11)

## Context

A task under `.taskgate/{human,ai,shared}/` receives `[args...]` verbatim as
its own argv (`cmd/run.go`, `cmd/ai.go`). Every task that wants named
positional arguments, flags, choices, defaults, or required-argument checks
must re-implement that parsing by hand — painful and inconsistent in
`sh`/`bash`, where `getopts` is clumsy and validation is boilerplate. Nothing
declares a task's interface, so AI agents (`ai show`, `ai run`) cannot see
what arguments a task expects and must guess, there is no pre-execution
validation of a missing required argument or an out-of-range value, and
there is no generated `--help`.

Inspired by [mise's `usage` spec](https://usage.jdx.dev/), we want a task to
declare its CLI interface in its annotation front-matter, and have taskgate
parse and validate the invocation against that declaration before running
the script, passing the validated values in as `taskgate_*` environment
variables.

## Decision

1. **Two new optional annotation keys, `args` and `flags`.** Extending the
   front-matter format of [ADR-0001](0001-annotation-format.md), `args` is
   an ordered YAML list of positional-argument declarations and `flags` is a
   list of flag declarations:

   ```bash
   #!/usr/bin/env bash
   # ---
   # summary: Deploy to an environment.
   # args:
   #   - name: env
   #     help: Target environment
   #     choices: [staging, prod]
   #     required: true
   #   - name: files
   #     help: Files to upload
   #     variadic: true
   # flags:
   #   - name: --dry-run
   #     short: -n
   #     help: Skip side effects
   #     type: bool
   #   - name: --tag
   #     help: Release tag
   #     default: latest
   #     choices: [stable, latest]
   # ---
   ./scripts/deploy.sh
   ```

   Per-entry keys: an **arg** carries `name` (required), `help`, `choices`
   (list of strings), `required` (bool, default `false`), `default`
   (string), and `variadic` (bool, default `false`); at most one arg may be
   variadic, and it must be the last one. A **flag** carries `name`
   (required, must start with `--`), `short` (optional, `-` + one
   character), `help`, `type` (`bool` | `string`, default `string`),
   `choices`, and `default`. `required: true` and `default` are mutually
   exclusive on the same entry; a `default` must be one of `choices` when
   both are present; a required positional may not follow an
   optional/defaulted one; `type: bool` flags may not carry `choices` or
   `default`; and every entry `name` (and its derived variable name, see
   point 3) must be unique across all args and flags. Genuinely unknown
   per-entry keys are ignored (forward-compatible), consistent with the
   existing annotation policy.

2. **A present-but-malformed `args`/`flags` block is an error.** Like
   `before`/`after` ([ADR-0005](0005-task-dependency-lifecycle.md)) and
   unlike best-effort `summary`/`body`, a structurally broken `args`/`flags`
   block (not a list, an entry missing `name`, or one of the consistency
   rules above violated) makes `run`/`ai run` refuse to execute and exit
   `1`; `validate`/`ai validate` report it as a `spec-malformed` (envelope
   is not parseable YAML) or `spec-invalid` (parses, but violates a rule)
   finding. A cleanly absent `args`/`flags` key is unaffected.

3. **Validated values are injected as `taskgate_<name>` environment
   variables; the spec'd task receives an empty argv.** When a spec is
   present, argv is not forwarded to the task at all — every value crosses
   the boundary as an environment variable:
   - **Variable name** — `taskgate_` + the entry's `name` with any leading
     `--`/`-` stripped, lowercased, and every remaining non-alphanumeric run
     collapsed to a single `_` (leading/trailing `_` trimmed). `env` →
     `taskgate_env`; `--dry-run` → `taskgate_dry_run`. A collision between
     two derived names is a spec error (point 2).
   - **Value arg / value flag** — `taskgate_<name>=<value>`. Absent with a
     declared `default` injects the default; absent with no default leaves
     the variable **unset** (the body can test `-z "$taskgate_x"`).
   - **Bool flag** — always set, `taskgate_<name>=true` when present,
     `=false` otherwise, so `[ "$taskgate_dry_run" = true ]` is always safe.
   - **Variadic arg** — indexed and lossless: `taskgate_<name>_count=<N>`
     plus `taskgate_<name>_1 … taskgate_<name>_N`. Zero operands still set
     `taskgate_<name>_count=0` with no indexed variables, so a body can
     `for i in $(seq 1 "$taskgate_files_count")` without a special case.

   Example: `deploy prod a.txt "b c.txt" --dry-run` yields `taskgate_env=prod`,
   `taskgate_tag=latest`, `taskgate_dry_run=true`, `taskgate_files_count=2`,
   `taskgate_files_1=a.txt`, `taskgate_files_2=b c.txt`.

4. **Spec-less tasks are untouched.** If a task declares neither `args` nor
   `flags`, no parser runs and `[args...]` is forwarded to the task's argv
   exactly as before this feature — every existing task, and any task that
   prefers to parse its own arguments, keeps working unchanged.

5. **`--help`/`-h`, usage errors, and generated help.** For the root
   target's own invocation (after its spec parses cleanly): `--help`/`-h` —
   reserved unless the task declares a conflicting flag of the same name —
   prints generated help (summary, a `Usage:` synopsis, an `Arguments:`
   table, a `Flags:` table, then the task's `body`) to stdout and exits
   `0`, running nothing. A usage error (unknown flag, missing required arg,
   missing value for a value-flag, too many positionals with no variadic, a
   value outside `choices`) prints one line, `taskgate: <reason>`, plus the
   `Usage:` synopsis to stderr and exits `2`, running nothing — exit `2` is
   reserved for usage errors so it stays distinguishable from a task body's
   own non-zero exit. Recognized invocation forms are `--flag`,
   `--flag value`, and `-n` (short); `--flag=value` and bundled short flags
   are not supported in v1.

6. **Only the root target is parsed.** The spec applies solely to the task
   named on the command line, consistent with the existing rule that only
   the root target receives arguments
   ([ADR-0005](0005-task-dependency-lifecycle.md)). `before`/`after`
   dependencies receive no arguments and are never parsed, regardless of
   whether they declare their own `args`/`flags`. Spec parsing happens
   before dependency-lifecycle execution, so a usage error on the root
   target aborts the whole invocation before any `before` dependency runs.

7. **`ai show`/`show` surface the spec.** `ai show <name>` JSON gains
   optional `args`/`flags` arrays (omitted entirely when the task has no
   spec) mirroring the declaration, so an agent can construct a correct
   invocation without guessing. Human `show` prints a single `Usage:` line
   under the summary. This is purely additive to the existing wire format
   ([ADR-0003](0003-ai-output-wire-format.md)).

## Consequences

- Authors get declarative argument/flag parsing, validation, and generated
  `--help` for free, and both `show` and AI agents can see a task's
  interface without reading its script body.
- Because the spec'd task's argv is always empty, a script that wants both
  the injected `taskgate_*` variables **and** raw argv access cannot have
  both — declaring a spec is an all-or-nothing switch from "parse argv
  yourself" to "read validated environment variables."
- The three-way exit-code split (`0` help / `2` usage error / `1` malformed
  spec, vs. whatever the task body itself returns) means callers and CI
  scripts can distinguish "the caller misused the CLI" from "the task ran
  and failed" from "the task's own annotation is broken" without parsing
  stderr text.
- Rich type coercion beyond `bool`/string (e.g. `int`, `path`, `date`),
  `=`-joined long flags, bundled short flags, negatable (`--no-foo`) flags,
  repeated flags, subcommands, and mutually-exclusive groups are out of
  scope for v1; `choices` is the only value constraint.
- **Shell completion generation is deferred to v2.** The machine-readable
  `ai show` spec (point 7) is the forward-compatible seam a future
  completion generator would read from; nothing in this decision needs to
  change to add it later.
- Passing parsed values down to `before`/`after` dependencies is out of
  scope, consistent with dependencies never receiving arguments at all
  (ADR-0005); a dependency that needs configuration must read it from its
  own environment or files, not from the root target's spec.
