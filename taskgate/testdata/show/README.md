# Show subcommand E2E suite

Every `*.txtar` file in this directory is one acceptance scenario for
`taskgate show` / `taskgate ai show`. The scenario's header comment
restates the contract clause it pins (exit code, envelope shape, edge-case
rule).

This suite is run by `TestShow` in `taskgate/main_test.go` via
`github.com/rogpeppe/go-internal/testscript`. Each `.txtar` re-executes the
test binary as the program `taskgate`, observing real stdout, stderr, and
exit codes.

## API stability

Exit codes are stable across this feature; consumers may switch on the
numeric value:

| Code | Meaning                                  | Pinned in                  |
|------|------------------------------------------|----------------------------|
| 0    | success                                  | most scenarios             |
| 1    | generic failure (incl. unknown command)  | legacy_list_removed        |
| 2    | invalid input                            | reject_filesystem_path     |
| 3    | not found                                | not_found                  |
| 4    | collision                                | collision                  |
| 5    | workspace missing (`.taskgate/` not found) | workspace_missing        |

The AI envelope schema is **additive**: future releases may add fields;
removing or renaming a field is a breaking change.

Consumers MUST ignore unknown fields in the AI envelope, and SHOULD treat
unrecognized top-level `kind` values as a forward-compat soft failure
(fall through to a generic "unsupported envelope" error).

## Where the AI envelope shapes are defined

Each shape lives in the header of its representative scenario:

- `listing`   → `browse_root_ai.txtar`
- `task`      → `inspect_task_ai.txtar` (with `inspect_task_ai_null_summary.txtar` pinning the null-vs-omit rule)
- `directory` → `dir_ai.txtar`
- `error`     → `collision.txtar` (the AI-form `error` envelope is documented there; the AI invocation itself is currently exercised only via the human form)

## Naming convention

File names describe the behaviour under test. Sequence numbers (`01_`,
`02_`) and spec labels (`us1_3`, `edge_5`) are intentionally not used —
the file name carries the meaning. AI-form variants of a base behaviour
use the `_ai` suffix.
