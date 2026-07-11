# Design: Task CLI Parser (usage-style arg/flag declarations)

Date: 2026-07-11
Status: Approved (pending implementation plan)

## Problem

A task under `.taskgate/{human,ai,shared}/` receives `[args...]` verbatim as its
own argv (`cmd/run.go`, `cmd/ai.go`). Every task that wants named positional
arguments, flags, choices, defaults, or required-argument checks must
re-implement that parsing by hand — painful and inconsistent in `sh`/`bash`,
where `getopts` is clumsy and validation is boilerplate. Nothing declares a
task's interface, so:

- AI agents (`ai show`, `ai run`) cannot see what arguments a task expects and
  must guess.
- There is no pre-execution validation: a missing required argument or an
  out-of-range value only surfaces (if at all) inside the script.
- There is no generated `--help`.

Inspired by [mise's `usage` spec](https://usage.jdx.dev/), we want a task to
**declare its CLI interface in its annotation front-matter**, and have taskgate
**parse and validate** the invocation against that declaration before running
the script, passing the validated values in as `taskgate_*` environment
variables.

## Vocabulary

Extends [`docs/show/glossary.md`](../../show/glossary.md).

- **arg spec** — the `args`/`flags` declaration in a task's annotation
  front-matter describing its CLI interface.
- **positional argument (`arg`)** — an ordered, name-bearing operand. The last
  one may be **variadic** (absorbs zero-or-more trailing operands).
- **flag** — a `--name` option, either **bool** (presence ⇒ `true`) or
  **value-taking** (`--tag latest`). May carry a single-character `short`.
- **choices** — an allowed-value set for an arg or value-flag; anything outside
  it is a usage error.
- **injected variable** — a `taskgate_<name>` environment variable carrying a
  parsed, validated value into the task body.
- **root target** — the task named on the command line. Per the existing
  dependency model ([ADR-0005](../../show/adr/0005-task-dependency-lifecycle.md)),
  it is the only task that receives arguments, hence the only task an arg spec
  applies to.

## Scope

In scope (v1):

- `args` / `flags` declaration in the annotation front-matter.
- Positional args with `required` / `default`; a trailing `variadic` arg.
- Flags: `bool` and value-taking, optional `short`, `choices`, `default`.
- `choices` validation for args and value-flags.
- Parse + validate at run time for **both** `taskgate run` and `taskgate ai
  run`; on failure, run nothing and exit non-zero.
- Validated values injected as `taskgate_*` environment variables.
- Generated `--help` / `-h` text (from the spec + summary/body).
- Static linting of the arg spec in `taskgate validate` / `ai validate`.
- `ai show <name>` JSON exposes the arg spec; human `show` prints a usage line.

Out of scope (YAGNI / deferred):

- **Shell completion generation** (bash/zsh). Deferred to v2; the machine-
  readable `ai show` spec is the forward-compatible seam for it.
- Rich type coercion beyond `bool` / string (no `int`/`path`/`date` types). A
  value is a string; `choices` is the only value constraint in v1.
- Subcommands, mutually-exclusive groups, repeated (`--x a --x b`) flags,
  negatable flags (`--no-foo`), `=`-joined long flags (`--tag=latest`).
- Passing parsed values to dependencies (only the root target is parsed).
- Environment-variable *sources* for flags (mise's `env=` on a flag).

## Design

### 1. Front-matter shape

Extend the existing annotation block
([ADR-0001](../../show/adr/0001-annotation-format.md)) with two optional keys,
`args` (an ordered list) and `flags` (a list):

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

Per-entry keys:

- **arg**: `name` (required, identifier), `help`, `choices` (list of strings),
  `required` (bool, default `false`), `default` (string), `variadic` (bool,
  default `false`).
- **flag**: `name` (required, must start with `--`), `short` (optional,
  `-` + single char), `help`, `type` (`bool` | `string`, default `string`),
  `choices`, `default`.

Rules (enforced at parse and by `validate`, §2.4):

- Only the **last** `arg` may be `variadic`; at most one variadic.
- `required: true` and `default` are mutually exclusive on the same entry.
- A `default` (or `choices`) must be consistent: `default` ∈ `choices` when both
  present.
- A required positional may not follow an optional/defaulted positional
  (otherwise positional binding is ambiguous).
- `type: bool` flags take no value: `choices`/`default` (other than an implicit
  `false`) are not allowed on them.
- `name` values, and their derived variable names (§2.2), must be unique across
  all args and flags.

**Strictness.** Like `before`/`after`
([ADR-0005](../../show/adr/0005-task-dependency-lifecycle.md)) and unlike
best-effort `summary`/`body`, a **present but malformed** `args`/`flags` block
is an error: `run`/`ai run` refuse to execute and exit non-zero, and `validate`
reports a finding. A cleanly absent key is fine. Genuinely unknown per-entry
keys are ignored (forward-compatible), consistent with the existing annotation
policy.

### 2. Behavior

#### 2.1 Spec-less tasks are untouched (backward compatibility)

If a task declares **neither** `args` **nor** `flags`, the parser is skipped
entirely and `[args...]` is forwarded to the task's argv exactly as today. This
preserves every existing task and keeps the raw-passthrough contract for tasks
that want to parse their own arguments.

#### 2.2 Value injection (`taskgate_<name>`)

When a spec **is** present, parsed values are passed **only** as environment
variables; the spec'd task receives an **empty argv**.

- **Variable name** — `taskgate_` + the entry `name` with any leading `--`/`-`
  stripped, lowercased, and every remaining non-alphanumeric run collapsed to a
  single `_`. `env` → `taskgate_env`; `--dry-run` → `taskgate_dry_run`.
  Collisions are a `validate`/parse error (§2.4).
- **Value arg / value flag** — `taskgate_<name>=<value>`. If absent and a
  `default` is declared, the default is injected. If absent with no default,
  the variable is **unset** (the body can test `-z`).
- **Bool flag** — always set: `taskgate_<name>=true` when present, `=false`
  otherwise (so `[ "$taskgate_dry_run" = true ]` is safe).
- **Variadic arg** — indexed, lossless, space-safe:
  `taskgate_<name>_count=<N>` and `taskgate_<name>_1 … taskgate_<name>_N`. Zero
  operands ⇒ `taskgate_<name>_count=0` and no indexed variables.

Example: `deploy prod a.txt "b c.txt" --dry-run` yields
`taskgate_env=prod`, `taskgate_tag=latest`, `taskgate_dry_run=true`,
`taskgate_files_count=2`, `taskgate_files_1=a.txt`, `taskgate_files_2=b c.txt`.

These are added to the child environment alongside the existing
`TASKGATE_PROJECT_ROOT` (`taskEnv` in `cmd/run.go`).

#### 2.3 Parse / validate at run time

For the root target only, after the spec parses cleanly:

- `--help` / `-h` (reserved unless the task declares a conflicting flag) →
  print generated help (§2.5) to stdout, exit `0`, run nothing.
- Bind flags (long or `short`, in any position) and positionals (in order; the
  trailing variadic absorbs the rest). Recognized forms: `--flag`,
  `--flag value`, `-n`. (`--flag=value` and bundled shorts are out of scope.)
- Validation errors — unknown flag, missing required arg, missing value for a
  value-flag, too many positionals (no variadic), a value outside `choices` —
  print a one-line reason plus a short `Usage:` line to **stderr**, exit **2**,
  run nothing. Exit `2` distinguishes a usage error from a task's own non-zero
  body exit.
- On success, build the injected environment (§2.2) and execute the body.

The spec applies to the **root target only**. Dependencies (`before`/`after`)
receive no arguments and are not parsed, consistent with
[ADR-0005](../../show/adr/0005-task-dependency-lifecycle.md). Parsing happens
before dependency lifecycle execution so a usage error aborts the whole
invocation before any `before` dependency runs.

#### 2.4 Static linting (`validate` / `ai validate`)

`internal/validate` walks each task and reports arg-spec authoring problems as
`Finding`s, in the same pipeline and human/AI renderers as the existing checks.
New finding conditions:

- malformed `args`/`flags` block (not a list; entry missing `name`);
- flag `name` not starting with `--`, or malformed `short`;
- `required: true` combined with `default`;
- `default` not in `choices`;
- variadic not last, or more than one variadic;
- required positional after an optional/defaulted positional;
- `bool` flag carrying `choices`/`default`;
- duplicate arg/flag `name` or colliding derived variable name.

#### 2.5 `--help` and usage text

Generated from the spec:

```
Deploy to an environment.                     # summary

Usage: taskgate run deploy [flags] <env> [files...]

Arguments:
  <env>        Target environment  (choices: staging, prod)
  [files...]   Files to upload

Flags:
  -n, --dry-run   Skip side effects
      --tag       Release tag  (choices: stable, latest; default: latest)
  -h, --help      Show this help
```

`body`, if present, is appended below. The one-line usage error form is
`taskgate: <reason>` + the `Usage:` line.

### 3. `ai show` integration

`ai show <name>` (`internal/show/render_ai.go`, `taskEnvelope`) gains an
optional `args` and `flags` array so an agent can construct a correct
invocation. Shape (omitted entirely when the task has no spec):

```json
{
  "kind": "task",
  "name": "deploy",
  "args": [
    {"name": "env", "help": "Target environment",
     "choices": ["staging", "prod"], "required": true, "variadic": false},
    {"name": "files", "help": "Files to upload", "variadic": true}
  ],
  "flags": [
    {"name": "--dry-run", "short": "-n", "type": "bool", "help": "Skip side effects"},
    {"name": "--tag", "type": "string", "help": "Release tag",
     "choices": ["stable", "latest"], "default": "latest"}
  ]
}
```

Human `show` prints the single `Usage:` line (§2.5) under the summary. This is a
purely additive change to the wire format
([ADR-0003](../../show/adr/0003-ai-output-wire-format.md)).

### 4. Package structure

New package **`taskgate/internal/cliparse`** owns the spec model, the argv
parser, help rendering, and env-map construction — parallel to how
`internal/taskgraph` owns the dependency model.

- Spec model + accessor: `annotationDoc` gains `args`/`flags`; a new accessor
  (e.g. `annotation.ParseSpec`) returns the parsed spec plus a `*Diagnostic`,
  leaving `Parse`/`ParseStrict`/`ParseDeps` behavior unchanged.
- `cliparse.Spec` — the validated model (ordered args, flags, derived variable
  names). A `Validate()` method returns the structural problems of §2.4 so both
  run-time and `validate` share one source of truth.
- `cliparse.Parse(spec, argv) (Result, error)` — binds an invocation, returning
  either the injected `map[string]string` (env additions) + a `helpRequested`
  signal, or a usage error carrying the reason and usage line.
- `cliparse.Help(spec, name) string` — renders §2.5.

Consumers:

- `cmd/run.go` / `cmd/ai.go` — after resolving the root target and before
  `taskgraph.Execute`, load its spec; if present, run `cliparse.Parse`, handle
  help/usage-error/exit, and merge the env additions into `taskEnv` for the
  root task only.
- `internal/validate` — calls `Spec.Validate()` per task to emit findings.
- `internal/show` — reads the spec for `ai show` JSON and the human usage line.

**Rejected alternatives.**

- *Reuse cobra/pflag per task* — a synthetic `FlagSet` cannot cleanly express
  positional `choices`, a named variadic tail, or `taskgate_*` env output, and
  its GNU-style ordering diverges from this declarative model. Rejected as an
  impedance mismatch.
- *Adopt the KDL `usage` spec verbatim* — introduces a second front-matter
  format and a KDL parser dependency into a YAML/cobra codebase. Rejected;
  extending the existing YAML annotation keeps a single source of truth.
- *Vendor the `usage` CLI (Rust)* — would require shelling out to a foreign
  binary. Rejected as a maintenance burden.

### 5. Testing

- **unit (`internal/annotation`)** — `args`/`flags` parsing; absent vs. present
  vs. malformed; unknown per-entry keys ignored.
- **unit (`internal/cliparse`)** — required-missing, choices violation, default
  injection, bool present/absent, variadic indexing (incl. zero and
  space-containing values), unknown flag, short-flag binding, too-many
  positionals, `--help` request, variable-name derivation and collisions.
- **e2e (testscript, `tests/e2e/run` and `tests/e2e/airun`)** — golden coverage
  for: `--help` output; a usage error (exit 2, nothing run); env injection via a
  task that echoes `taskgate_*`; **spec-less passthrough** (unchanged argv);
  a variadic task iterating the indexed variables.
- **validate** — golden updates for each new finding kind (human and AI forms).

### 6. Documentation

- New ADR under `docs/show/adr/` recording the arg-spec model, the
  env-injection contract, and the empty-argv / spec-less-passthrough decision.
- Glossary additions for the vocabulary above; a note in `docs/show/requirements.md`
  capturing the run-time parse/validate guarantee.
- Update `taskgate/internal/usage/guide.md` (per the guide-sync policy): document
  the `args`/`flags` annotation and that `run`/`ai run` validate the invocation
  and inject `taskgate_*` variables.
