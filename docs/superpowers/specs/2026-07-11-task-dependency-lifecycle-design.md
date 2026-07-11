# Design: Task Dependency Auto-Execution (before/after lifecycle)

Date: 2026-07-11
Status: Approved (pending implementation plan)

## Problem

Task files under `.taskgate/{human,ai,shared}/` currently run in isolation:
`taskgate run <task>` executes exactly one script. There is no way to declare
that a task requires other tasks to run first, or that cleanup tasks should run
after it. Authors must chain prerequisites by hand (calling `taskgate run`
from inside a script), which is error-prone and invisible to `taskgate show`
and `taskgate validate`.

We want to declare a task's dependencies in its annotation front-matter and
have `taskgate run` / `taskgate ai run` execute them automatically, with a
per-task `before`/`after` lifecycle.

## Vocabulary

Extends [`docs/show/glossary.md`](../../show/glossary.md).

- **before dependency** — a task name listed under the `before` key of a task's
  annotation block. Runs before the task's own body.
- **after dependency** — a task name listed under the `after` key. Runs after
  the task's own body, **only if that body succeeded**.
- **task lifecycle** — for any task `T`, execution is:
  `all before-deps (recursively) → T's body → (if body succeeded) all after-deps (recursively)`.
- **root target** — the task named on the command line. It is the only task
  that receives the user-supplied arguments.
- **dependency graph** — the directed graph reachable from the root target via
  `before` and `after` edges.

## Scope

In scope:

- `before` / `after` declaration in the annotation front-matter.
- Automatic lifecycle execution for **both** `taskgate run` (human+shared) and
  `taskgate ai run` (snapshot + freshness).
- Cycle / unknown-reference / non-executable detection at run time (refuse to
  execute anything) and statically in `taskgate validate` / `taskgate ai
  validate`.

Out of scope (YAGNI):

- Timestamp/content-based incremental rebuild or caching (make-style
  up-to-date checks).
- Parallel execution of independent dependencies. Execution is sequential and
  deterministic.
- Passing arguments to dependencies (only the root target receives arguments).
- Conditional / guarded dependencies.

## Design

### 1. Front-matter shape

Extend the existing annotation block ([ADR-0001](../../show/adr/0001-annotation-format.md))
with two optional keys, `before` and `after`, each a YAML list of `run`-style
names (bare `build` or slash-nested `deploy/prod`):

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

- Absent or empty list → no dependencies of that kind.
- Names follow the same `run`-style grammar as `taskgate run` targets:
  bare or slash-separated logical names, never filesystem paths.

**Strictness divergence from FR-009.** `summary` and `body` are best-effort:
a malformed annotation is treated as empty. `before`/`after` are **not**
best-effort, because silently dropping a malformed dependency list would skip a
prerequisite and change execution semantics. When a `before`/`after` key is
**present but malformed** (e.g., not a list, or contains a non-string entry),
`run`/`ai run` refuse to execute anything and exit non-zero, and `validate`
reports it as a finding. A cleanly absent key is fine.

### 2. Execution model (immediate `after`, recursive, deduplicated)

Execution is a depth-first traversal with memoization and stack coloring.

Per-task states: `UNVISITED`, `ON_STACK` (cycle sentinel), `DONE(success)`,
`DONE(failed)`.

```
run(T, isRoot):
  if T is DONE:            return memoized result       # dedup
  if T is ON_STACK:        cycle (pre-detected; safety net)
  mark T ON_STACK
  for d in T.before (declared order):
    if run(d, false) == failed:
      mark T DONE(failed)
      return failed                                     # skip remaining before, body, after
  ok = execute(T.body, args if isRoot else none)
  if not ok:
    mark T DONE(failed)
    return failed                                       # skip after
  for d in T.after (declared order):
    run(d, false)                                       # after-dep failure recorded globally
  mark T DONE(success)
  return success
```

Properties:

- **Deduplication** — keyed by resolved physical path. A task's body runs at
  most once per invocation, even when reached through multiple edges (diamond
  dependencies).
- **before failure** — aborts that task's body and `after`, and short-circuits
  its remaining `before` list; the failure propagates to the parent, which in
  turn skips its own body and `after`.
- **body failure** — skips that task's `after` (satisfies "only run the
  `after` of tasks that succeeded").
- **overall exit code** — the exit code of the first task that fails in
  execution order; `0` if none fails. A `before` failure that prevents the root
  target from running still yields a non-zero exit.
- **Intentional consequence of the immediate model + dedup** — a dependency's
  `after` runs the moment that dependency succeeds, which can be *before* a
  later task that also depends on it. Example: with `deploy(before=[build],
  after=[notify])` and `build(after=[clean])`, the order is
  `build → clean → deploy → notify`. This is the documented, intended behavior
  of the chosen immediate model, not a bug.

### 3. Name resolution (audience-specific)

Dependency names resolve within the invoking command's audience view, using the
same precedence as the corresponding run command:

- **`taskgate run`** — resolve each dependency across `.taskgate/human/` then
  `.taskgate/shared/` (audience-first), identical to the existing
  `resolveHumanTask`. A missing or non-executable dependency is an error.
- **`taskgate ai run`** — resolve each dependency inside the **snapshot
  directory** (identical to the existing `resolveAITask`), and freshness-check
  every resolved dependency against its `.taskgate/{ai,shared}/` source
  (identical to `checkSnapshotFresh`). If any dependency in the graph is stale,
  emit the existing "snapshot out of date" error and execute nothing.

Dependencies resolve only within the invoking audience's view; cross-audience
references are not supported (a `shared` task cannot depend on a `human`-only
task and still work under `ai run`). An unresolvable name is an unknown-
reference error (§4).

### 4. Detection: cycles, unknown references, non-executable

- **At run time (`run` / `ai run`)** — before executing anything, build the
  dependency graph reachable from the root target across both `before` and
  `after` edges. Detect **unknown reference** (name does not resolve),
  **non-executable target**, **malformed `before`/`after` field** (§1), and
  **cycle** (via stack coloring). On any of these, execute nothing and exit
  non-zero with a message identifying the offending task(s).
- **Statically (`validate` / `ai validate`)** — walk every task's `before`/
  `after` across the audience's buckets and report the same conditions as
  `Finding`s. New finding kinds are added for: unknown dependency reference,
  cycle, non-executable dependency target, and malformed `before`/`after`
  field. This extends the existing validate pipeline
  ([`internal/validate`](../../../taskgate/internal/validate)) and its
  human/AI renderers.

### 5. Package structure (recommended approach)

New package `taskgate/internal/taskgraph` owns graph construction, detection,
and lifecycle orchestration. A `Resolver` seam lets each caller supply its own
name resolution so the graph logic is shared, not duplicated.

- `Resolver` interface: `Resolve(name string) (path string, err error)`.
  - `run` supplies a human+shared resolver.
  - `ai run` supplies a snapshot resolver that also freshness-checks.
  - `validate` supplies a resolver over the audience's buckets for static
    linting.
- `Build(target string, r Resolver) (*Graph, error)` — recursively resolves and
  parses `before`/`after`, returning the graph or a detection error
  (unknown / non-executable / malformed / cycle).
- `Execute(g *Graph, rootArgs []string, run Runner) (exitCode int)` — drives the
  lifecycle, dedup, and exit-code rules of §2. `Runner` is a callback that
  executes a resolved task path with the given args (the callers wire in the
  existing `exec.Command` plumbing, including `TASKGATE_PROJECT_ROOT`).
- Annotation parsing is extended: `annotationDoc` gains `before`/`after`
  fields, exposed through a new accessor so existing `Parse`/`ParseStrict`
  behavior for `summary`/`body` is unchanged.

Consumers:

- `cmd/run.go` builds a human+shared resolver and runner, then delegates to
  `Build` + `Execute`.
- `cmd/ai.go` builds a snapshot resolver (with freshness) and runner, then
  delegates.
- `internal/validate` calls `Build` (detection only) per task to emit findings.

**Rejected alternatives.**

- *Inline in `cmd`* — graph build/execute in `run.go` with `validate`
  re-implementing detection. Rejected: duplicates the graph logic.
- *Generic build engine* — make-style timestamp caching and up-to-date checks.
  Rejected as YAGNI for the current requirement.

### 6. Testing

- **e2e (testscript, `tests/e2e/`)** for both `run` and `ai run`:
  - linear chain runs in order;
  - diamond dependency runs the shared node once (dedup);
  - `before` failure aborts the target and its `after`;
  - body failure skips its own `after` while a succeeded `before`'s `after`
    has already run;
  - cycle → error, nothing executed;
  - unknown reference → error, nothing executed;
  - `ai run` with a stale dependency → snapshot-out-of-date error.
- **unit (`internal/taskgraph`)** — graph build, cycle detection, dedup order,
  exit-code selection, immediate-`after` ordering.
- **unit (`internal/annotation`)** — `before`/`after` parsing, empty/absent
  vs. malformed.
- **validate** — golden updates for the new finding kinds (human and AI forms).

### 7. Documentation

- New ADR under `docs/show/adr/` (or a sibling area) recording the
  dependency/lifecycle model and the immediate-`after` decision.
- Glossary additions for the vocabulary in this document.
- Requirements note capturing the run-time and validate detection guarantees.
