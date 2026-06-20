# ADR-0004: Retire legacy `list` and `ai list` subcommands

**Status**: Accepted (2026-06-16)

## Context

The new `taskgate show` and `taskgate ai show` subcommands subsume the discovery use case that `taskgate list` / `taskgate ai list` served, plus a strictly larger surface (summaries, bodies, directory navigation). The question is what to do with the old subcommand names — keep as aliases, ship a deprecation shim, or just delete.

The user explicitly asked for "no backwards compatibility" for callers of the old `list` output. Recorded in the original spec's Naming note and Assumptions; the rationale lives here now.

## Decision

**Delete `list` and `ai list` outright.** No alias, no deprecation shim, no migration notice in the binary. Documented in the PR description and the release notes; the removal is also pinned by `taskgate/testdata/show/legacy_list_removed.txtar`.

## Consequences

**Positive**:

- Codebase free of dead-or-aliased commands and the ongoing maintenance surface they imply.
- Callers see `taskgate --help` listing only `show` after the change — the strongest discoverability signal.

**Negative**:

- Downstream callers using `taskgate list` get an "unknown command" error with no built-in pointer to `show`. Mitigated by release notes; not mitigated in the binary itself.

## Alternatives considered

- **Alias `list` → `show` for one release**: explicitly rejected by the user.
- **Print a deprecation message and call through to `show`**: rejected. The surfaces differ in non-trivial ways (sort order, audience-bucket visibility, exit codes on collision), so a transparent alias would surprise callers without actually helping them.
