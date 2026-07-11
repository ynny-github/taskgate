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

## Check task files
`taskgate ai validate`          — check every task under .taskgate/
`taskgate ai validate <name>`   — check just one task or directory

Reports authoring problems: a missing execute bit, a missing shebang, a
broken annotation envelope, a name that collides between the shared and ai
buckets, or a dependency problem — an unknown `before`/`after` name, a
non-executable dependency, a malformed `before`/`after` list, or a
dependency cycle. Output is a single JSON object:
`{"kind":"validation","ok":<bool>,"findings":[...]}`. `ok` is true with an
empty `findings` list when everything is valid; exit is non-zero when any
finding is present. Fix the reported files before running the task.

## Task metadata (annotations)
Each task can carry a summary/body annotation, written as YAML
front-matter inside a leading comment block of the script itself.
Delimit with `---` lines; every line uses the script's own comment
prefix (`#` for sh, `//` for JS/Go, `--` for Lua, `;` for Lisp).

Recognized keys: `summary` (single line), `body` (multi-line via `|`),
`before` (list of task names), `after` (list of task names). Genuinely
unknown keys are ignored (forward-compatible), but a `before`/`after` that is
present yet malformed (not a list of task names) is an error — the task
refuses to run until you fix it.

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
