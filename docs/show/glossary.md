# Glossary

Terms used across the taskgate spec, ADRs, tests, and CLI documentation. The canonical definition for each term lives here; other documents reference these names.

## Task entry

A single executable file under `.taskgate/{human,ai,shared}/...`. Has a path; may carry an annotation block providing a summary and an optional body. A task's physical location determines which audience(s) can see it via show.

## Directory entry

A folder under `.taskgate/`. Has a path; carries no summary or body; lists its immediate children (which are themselves task entries or directory entries).

## Audience bucket

One of the three reserved top-level directories under `.taskgate/`: `human/`, `ai/`, `shared/`. Acts as an internal audience classifier and is **invisible** in show output — never appears as its own row. Tasks under `shared/` are visible to both audiences; tasks under `human/` are visible only to `taskgate show`; tasks under `ai/` are visible only to `taskgate ai show`.

## Merged view

The user-facing tree that show exposes. For `taskgate show` it is the union of entries directly under `.taskgate/shared/` and `.taskgate/human/`; for `taskgate ai show` it is the union of `.taskgate/shared/` and `.taskgate/ai/`. The buckets themselves are folded out; entry paths retain their real physical location.

## Annotation block

The summary + optional body, expressed in the project's chosen comment notation, embedded at the top of a task file. Two parts: a single-line **summary** and a free-form multi-line **body**. Format is defined in ADR-0001.

## Output record

The unit of output. Carries the entry's real physical path, its summary (possibly empty), its body (only for the single-target file case), and — for a directory target — the list of immediate child records (path + summary only) in the merged view. In the AI form each record and the file/directory envelopes also carry a `name`: the entry's `run`-style name (its physical `path` minus the `.taskgate/<bucket>/` prefix).

## Audience mode

The output shape and the filter applied. **Human** mode (`taskgate show`) emits formatted text and merges `shared/` ∪ `human/`. **AI** mode (`taskgate ai show`) emits a structured form (see ADR-0003) and merges `shared/` ∪ `ai/`. Both carry the same set of entries for any given invocation; the AI form additionally reports each entry's full physical path, while the human form shows basenames in its recursive/directory tree (ADR-0004).

## Before dependency

A task name listed under the `before` key of a task's annotation block. Runs before the task's own body.

## After dependency

A task name listed under the `after` key of a task's annotation block. Runs after the task's own body, **only if that body succeeded**.

## Task lifecycle

For any task `T`, execution is: all before-deps (recursively) → T's body → (if body succeeded) all after-deps (recursively).

## Root target

The task named on the command line. It is the only task that receives the user-supplied arguments.

## Dependency graph

The directed graph reachable from the root target via `before` and `after` edges.
