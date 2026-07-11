# Using taskgate (for AI agents)

This project uses taskgate to run project tasks. Use these commands
instead of guessing raw scripts.

## Discover tasks
`taskgate ai show`          — list every task and directory (whole tree)
`taskgate ai show <name>`   — inspect one entry: a task's summary, body,
                              and path, or a directory's immediate children

## Run a task
`taskgate ai run <name> [args...]`
- <name> is a bare or slash-separated task name (e.g. `build`, `deploy/prod`).
- Filesystem paths are NOT accepted.
- If the task declares dependencies (see below), they run automatically —
  you only invoke the target. `[args...]` go to the target task only, never
  to its dependencies.

## Task dependencies (before/after)
A task can declare other tasks to run around it, in its annotation:
- `before` — tasks that must run first. If any fails, the target and its
  `after` are skipped.
- `after` — tasks that run afterward, but only if the target's own body
  succeeded.
Dependencies are followed recursively and each task runs at most once per
invocation. A dependency's `after` runs as soon as that dependency
succeeds — so a shared dependency's `after` can run before a later task that
also depends on it. A dependency cycle, an unknown dependency name, or a
non-executable dependency is an error: nothing runs until you fix it.

## Task arguments (args/flags)
A task can declare a CLI interface — named positional arguments and flags —
in its annotation, instead of leaving `[args...]` to raw shell parsing:
- `args` — an ordered list of positionals. Each entry has `name`, optional
  `help`, `choices` (allowed values), `required` (bool), `default`
  (string), and `variadic` (bool; only the last arg may be variadic,
  absorbing zero-or-more trailing operands).
- `flags` — a list of `--name` options. Each entry has `name` (must start
  with `--`), optional `short` (`-` + one character), `help`, `type`
  (`bool` or `string`, default `string`), `choices`, and `default`.

Example:
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
    #     type: bool
    #     help: Skip side effects
    # ---
    ./scripts/deploy.sh

When a task declares `args`/`flags`, `taskgate ai run <name> [args...]`
parses and validates the invocation **before** running anything:
- On success, the task itself receives an **empty** argv — every value is
  passed as an environment variable instead. A value arg/flag becomes
  `taskgate_<name>=<value>` (its declared `default` if omitted, unset if
  omitted with no default); a bool flag is always `taskgate_<name>=true` or
  `=false`; a variadic arg becomes `taskgate_<name>_count=<N>` plus
  `taskgate_<name>_1 … taskgate_<name>_N`.
- `--help`/`-h` prints generated help and exits `0`, running nothing.
- A usage error (unknown flag, missing required argument, a value outside
  `choices`, too many positionals, a missing flag value) exits `2` and
  prints the reason to stderr, running nothing.
- A malformed or otherwise invalid `args`/`flags` block refuses to run and
  exits `1` — check `ai validate` for the `spec-malformed`/`spec-invalid`
  finding.

A task with **no** `args`/`flags` is unaffected: `[args...]` is forwarded
to its argv exactly as before. Only the target task you name is parsed;
`before`/`after` dependencies never receive arguments.

Run `ai show <name>` to see a task's declared `args`/`flags` before
invoking it — construct the call from that instead of guessing.

## Check task files
`taskgate ai validate`          — check every task under .taskgate/
`taskgate ai validate <name>`   — check just one task or directory

Reports authoring problems: a missing execute bit, a missing shebang, a
broken annotation envelope, a name that collides between the shared and ai
buckets, a dependency problem — an unknown `before`/`after` name, a
non-executable dependency, a malformed `before`/`after` list, or a
dependency cycle — or an `args`/`flags` spec problem (`spec-malformed`:
the block isn't a list or an entry is missing `name`; `spec-invalid`: it
parses but breaks a rule, e.g. two variadic args, a default outside
`choices`, or a colliding variable name). Output is a single JSON object:
`{"kind":"validation","ok":<bool>,"findings":[...]}`. `ok` is true with an
empty `findings` list when everything is valid; exit is non-zero when any
finding is present. Fix the reported files before running the task.

## Task metadata (annotations)
Each task can carry a summary/body annotation, written as YAML
front-matter inside a leading comment block of the script itself.
Delimit with `---` lines; every line uses the script's own comment
prefix (`#` for sh, `//` for JS/Go, `--` for Lua, `;` for Lisp).

Recognized keys: `summary` (single line), `body` (multi-line via `|`),
`before` (list of task names), `after` (list of task names), `args` and
`flags` (see "Task arguments" above). Genuinely unknown keys are ignored
(forward-compatible), but a `before`/`after`/`args`/`flags` that is present
yet malformed is an error — the task refuses to run until you fix it.

Example (sh):
    #!/bin/sh
    # ---
    # summary: Deploy the project to production.
    # before:
    #   - build
    #   - test
    # after:
    #   - notify
    # ---
    ./scripts/deploy.sh

## Snapshot safety
AI tasks run from an approved snapshot. If you see
"snapshot ... is out of date", STOP and ask a human to run
`taskgate snapshot install` to review and approve the changes.
Do not attempt to bypass it.
