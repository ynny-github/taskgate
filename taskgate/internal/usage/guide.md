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

## Check task files
`taskgate ai validate`          — check every task under .taskgate/
`taskgate ai validate <name>`   — check just one task or directory

Reports authoring problems: a missing execute bit, a missing shebang, a
broken annotation envelope, or a name that collides between the shared and
ai buckets. Output is a single JSON object:
`{"kind":"validation","ok":<bool>,"findings":[...]}`. `ok` is true with an
empty `findings` list when everything is valid; exit is non-zero when any
finding is present. Fix the reported files before running the task.

## Task metadata (annotations)
Each task can carry a summary/body annotation, written as YAML
front-matter inside a leading comment block of the script itself.
Delimit with `---` lines; every line uses the script's own comment
prefix (`#` for sh, `//` for JS/Go, `--` for Lua, `;` for Lisp).

Recognized keys: `summary` (single line), `body` (multi-line via `|`).
Unknown keys are ignored (forward-compatible).

Example (sh):
    #!/bin/sh
    # ---
    # summary: Build the project for the current platform.
    # body: |
    #   Reads VERSION from the environment.
    # ---
    go build ./...

## Snapshot safety
AI tasks run from an approved snapshot. If you see
"snapshot ... is out of date", STOP and ask a human to run
`taskgate snapshot install` to review and approve the changes.
Do not attempt to bypass it.
