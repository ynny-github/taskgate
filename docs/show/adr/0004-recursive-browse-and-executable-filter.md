# ADR-0004: Recursive no-argument browse and executable-only listing

**Status**: Accepted (2026-07-06)

## Context

`taskgate show` previously listed only the immediate root level and let
directories carry a summary/body via an `_index` description file. Two
changes were requested: a no-argument `show` should reveal the whole
workspace, and files that cannot be run should not be advertised.

## Decision

1. **No-argument `show` walks the merged view recursively** and renders an
   indented tree (human) or a flat, full-path listing envelope (AI). An
   explicitly named directory still lists only its immediate children.
2. **The `_index` directory-description feature is removed.** Directories
   carry no summary/body, and `_index` loses its reserved status — it is an
   ordinary file, subject to the same rules as any other.
3. **Non-executable regular files are hidden** from listing and name
   resolution (`mode & 0o111 == 0`), matching `taskgate run`, which only
   runs executable tasks. Symlinks are judged by their resolved target's
   mode; escaping symlinks keep their FR-008 handling.

## Consequences

- The merged view has one recursive entry point (no argument) and one
  one-level entry point (explicit directory), which is a small asymmetry
  callers must learn.
- Collision detection now spans every level the recursive walk visits.
- The `validate` subcommand still recognizes `_index`; aligning it is a
  separate follow-up.
