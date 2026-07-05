# Using taskgate (for AI agents)

This project uses taskgate to run project tasks. Use these commands
instead of guessing raw scripts.

## Discover tasks
`taskgate ai show`          — list available tasks and directories
`taskgate ai show <name>`   — show one task/dir's summary, body, and path

## Run a task
`taskgate ai run <name> [args...]`
- <name> is a bare or slash-separated task name (e.g. `build`, `deploy/prod`).
- Filesystem paths are NOT accepted.

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
