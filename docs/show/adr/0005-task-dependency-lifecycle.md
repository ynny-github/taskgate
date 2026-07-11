# ADR-0005: Task dependency lifecycle (`before`/`after`)

**Status**: Accepted (2026-07-11)

## Context

Task files under `.taskgate/{human,ai,shared}/` ran in isolation: `taskgate
run <task>` executed exactly one script. There was no way to declare that a
task requires other tasks to run first, or that cleanup/notification tasks
should run after it. Authors chained prerequisites by hand (calling
`taskgate run` from inside a script), which is error-prone and invisible to
`taskgate show` and `taskgate validate`.

We want to declare a task's dependencies in its annotation front-matter and
have `taskgate run` / `taskgate ai run` execute them automatically, with a
per-task `before`/`after` lifecycle.

## Decision

1. **Two new optional annotation keys, `before` and `after`.** Each is a YAML
   list of `run`-style names (bare `build` or slash-nested `deploy/prod`,
   never filesystem paths), extending the front-matter format of
   [ADR-0001](0001-annotation-format.md):

   ```bash
   #!/usr/bin/env bash
   # ---
   # summary: deploy to prod
   # before:
   #   - build
   #   - test
   # after:
   #   - notify
   # ---
   ```

   An absent or empty list means no dependencies of that kind.

2. **Per-task lifecycle: `before → body → after-on-success`.** For any task
   `T`, execution is: all `before` dependencies (recursively) run first, then
   `T`'s own body, then — only if that body succeeded — all `after`
   dependencies (recursively). A `before` dependency's failure skips `T`'s own
   body and `after` entirely and propagates the failure to `T`'s caller.

3. **Recursive traversal, deduplicated by physical path.** The dependency
   graph reachable from the root target via `before`/`after` edges is walked
   depth-first with per-task states (`UNVISITED`, `ON_STACK`, `DONE(success)`,
   `DONE(failed)`). A task's body runs at most once per invocation — even when
   reached through multiple edges (diamond dependencies) — keyed by its
   resolved physical path, and its previously recorded outcome is reused
   (memoized) on later visits. `ON_STACK` doubles as a cycle sentinel; the
   graph is also checked for cycles up front during detection (see point 6).

4. **Immediate-`after` ordering consequence.** Because a dependency's `after`
   runs the instant that dependency's own body succeeds — not after the whole
   graph finishes — a dependency's `after` can run *before* a later,
   independent task that also depends on it. Worked example: task `deploy`
   declares `before: [build]` and `after: [notify]`; task `build` declares
   `after: [clean]`. Execution order is:

   ```
   build → clean → deploy → notify
   ```

   `clean` (the `after` of `build`) runs immediately once `build` succeeds,
   before `deploy`'s own body starts — not batched to the end after `notify`.
   This is the documented, intended behavior of the chosen immediate model,
   not a bug.

5. **Only the root target receives arguments.** The task named on the command
   line is the only one that receives the user-supplied arguments; every
   `before`/`after` dependency runs with no arguments, regardless of depth.

6. **Strictness divergence from FR-009 for malformed `before`/`after`.**
   `summary`/`body` remain best-effort per FR-009 (a malformed annotation is
   treated as empty). `before`/`after` are **not** best-effort: silently
   dropping a malformed dependency list would skip a prerequisite and change
   execution semantics, which is worse than refusing outright. When a
   `before`/`after` key is **present but malformed** (not a list, or contains
   a non-string entry), `run`/`ai run` refuse to execute anything and exit
   non-zero, and `validate` reports it as a finding. A cleanly absent key is
   unaffected and still best-effort (i.e., simply "no dependencies").

7. **Audience-scoped name resolution.** Dependency names resolve within the
   invoking command's own audience view, using the same precedence as that
   command's normal task resolution:
   - `taskgate run` resolves each dependency across `.taskgate/human/` then
     `.taskgate/shared/` (audience-first), identical to existing single-task
     resolution.
   - `taskgate ai run` resolves each dependency inside the **snapshot
     directory**, and freshness-checks every resolved dependency against its
     `.taskgate/{ai,shared}/` source. If any dependency in the graph is stale,
     the existing "snapshot out of date" error is emitted and nothing
     executes.
   Dependencies resolve only within the invoking audience's view; a `shared`
   task cannot depend on a `human`-only task and still work under `ai run`.

8. **Detection at both run time and `validate` time.** Before executing
   anything, `run`/`ai run` build the full dependency graph reachable from
   the root target across both `before` and `after` edges, and detect unknown
   references, non-executable targets, malformed `before`/`after` fields, and
   cycles. On any of these, nothing executes and the command exits non-zero
   identifying the offending task(s). `validate`/`ai validate` perform the
   same detection statically — walking every task's `before`/`after` across
   the audience's buckets — and report the same conditions as `Finding`s
   instead of refusing an execution that was never requested.

## Consequences

- Authors get declarative, ordered prerequisite/cleanup wiring without
  hand-rolled `taskgate run` calls inside scripts, and `show`/`validate` can
  reason about it statically.
- The immediate-`after` model is simple to implement (a single DFS with
  memoization) and gives a deterministic, sequential execution order, but it
  means `after` is "runs right after this dependency succeeds," not "runs
  after everyone who depends on it is done." Authors relying on the latter
  reading will be surprised until they see the worked example above.
- Because dependencies never receive arguments, a task designed to be run
  standalone with arguments cannot be reused unmodified as a `before`/`after`
  dependency if it requires arguments to do useful work.
- The strictness divergence means a single malformed `before`/`after` key
  anywhere in the graph blocks the entire invocation, which is a stronger
  failure mode than the rest of the annotation format; this is intentional
  given the correctness stakes of silently skipping a dependency.
- No caching or parallelism: dependencies always re-run in full, and
  execution is strictly sequential. This is accepted as YAGNI for the current
  requirement (make-style incremental builds are out of scope).
