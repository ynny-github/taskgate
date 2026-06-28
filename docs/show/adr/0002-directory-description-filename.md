# ADR-0002: Directory description filename

**Status**: Accepted (2026-06-16)

## Context

Authors should be able to attach a summary + body description to a directory under `.taskgate/`, in addition to the per-task annotations defined in ADR-0001. The directory description is **optional** — directories without one still appear in `taskgate show` output, just with no summary/body section.

We need a reserved filename for these description files that:

- Cannot collide with a legitimate task script name (because `taskgate run <name>` resolves bare names against file basenames).
- Is obviously a tooling artifact, not a task or a piece of human documentation.
- Is not hidden from `ls` by default (newcomers should see it).

## Decision

**`_index`** (no extension), located directly inside the directory it describes. Same YAML front-matter format as task annotations (see ADR-0001); the line-comment prefix is optional inside `_index` since the file is not executed as a script.

`_index` is never listed as a child entry of its containing directory.

## Consequences

**Positive**:

- Leading underscore is unambiguously "not a task" to operators, discouraging accidental `taskgate run _index`.
- Familiar Hugo-style precedent: `_index` is widely understood as "this folder, not its children".
- No extension keeps editors from auto-rendering it as Markdown — it reads as a tooling artifact.
- Optional executability: if `_index` happens to be a runnable script, it's still consumed only as a description source (not double-listed as a child).

**Negative**:

- The leading underscore is unusual outside Hugo-influenced ecosystems; some new contributors will need a single sentence of orientation.

## Alternatives considered

- **`README` / `README.md`**: overloaded — many repos already use READMEs for human documentation that doesn't follow this annotation format. Forcing the convention onto an existing filename is too constraining.
- **`.description` / `.taskgate.md`**: dotfile hides from `ls` by default, working against discoverability for new contributors.
- **`_taskgate-description`**: explicit but verbose; authors must type it more often than they'd like.
