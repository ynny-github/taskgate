# Feature Specification: Show Subcommand — Tasks and Directories with Descriptions

**Feature Branch**: `feature/show-index-subcommand`

**Created**: 2026-06-15

**Status**: Draft

**Input**: User description: "list 機能を拡張して、path の他に所定の表記に従ってコメントアウトでファイルに書かれている概要、本文を出力する機能が欲しい。出力の出しわけは、人と ai で受けるのに加えて、指定された path で出力を出し分けたい。ファイルが指定された場合は、ファイルの 概要と本文、どちらも出力。directory が指定された場合、その directory の概要と本文を出力し、子要素の概要を出力したい。directory には、directory の説明を記述できる専用のファイルを設置できるようにしたい。設置は optionalで。"

**Naming note**: The user originally framed this as "extend `list`", but as the scope settled it became clear the resulting command is no longer "just a listing" — it inspects, describes, and lets the user navigate targets. The feature therefore introduces a new `show` subcommand (`taskgate show` for humans, `taskgate ai show` for AI) and **retires** the current `list` / `ai list` subcommands. Backwards compatibility for callers of the old `list` output is explicitly out of scope.

## Clarifications

### Session 2026-06-16

- Q: When `taskgate show` is run with no path argument, what should the top-level output contain? (Story 1 acceptance scenarios said "every task"; FR-003c + FR-010 said "immediate children of `.taskgate/`", i.e. the three audience buckets — a contradiction.) → A: The audience buckets `human/`, `ai/`, `shared/` are **not** surfaced as their own entries. Instead, `show` presents a **merged audience-filtered view**: `taskgate show` merges entries from `.taskgate/shared/` and `.taskgate/human/`; `taskgate ai show` merges entries from `.taskgate/shared/` and `.taskgate/ai/`. Each row's path field reflects the task's real physical location under `.taskgate/` (e.g., `.taskgate/human/build`, `.taskgate/shared/lint`). This mirrors the existing `taskgate run` / `taskgate ai run` resolution model, where the audience-bucket directories are an internal classification, not a user-facing concept. The "immediate children only" rule (FR-010) still applies — but to the merged view, not to the physical `.taskgate/` tree.
- Q: When the user passes an explicit argument to show, what forms are accepted? → A: **`run`-style bare or nested names only**. Examples: `taskgate show build`, `taskgate show deploy/prod`. Filesystem paths — absolute, cwd-relative, or `.taskgate/`-prefixed — are **not** accepted as input. The user-facing namespace is fully unified with `run`; physical paths do not leak into the input surface. The output `path` field still reflects the real physical location of the resolved entry (per the previous clarification), so users can copy a real path for debugging, but they cannot paste it back as a show argument.
- Q: What concrete wire format should `taskgate ai show` use (envelope JSON vs NDJSON vs other)? → A: **Deferred to the plan phase.** The spec keeps the contract at "structured, parseable by a single parser across all three invocation shapes" (FR-006); the exact wire format (envelope JSON, NDJSON, etc.) will be locked down during `/speckit-plan` once the parser implementation constraints and the AI-consumer ergonomics have been considered together.
- Q: How should name collisions in the merged view be handled (same logical name present in both the audience bucket and the shared bucket)? → A: **Collisions are a hard error.** The show command does not allow same-name entries to coexist in the merged view. On detecting any collision in the region being shown, the command emits a warning identifying all conflicting real paths and exits with a non-zero status; no partial output of the conflicting region is emitted. This applies symmetrically to the no-argument browse, explicit name references, and directory navigation. Operators must resolve the collision at the source (rename or remove one of the entries). This makes show **stricter than `run`** (which silently picks the audience-bucket entry via precedence) — by design, because show's purpose is to describe; a silent collision would mislead readers.
- Q: What sort order should be used for the children of a directory (or for the merged root view)? → A: **Type-grouped, then basename alphabetical (case-sensitive).** Directories appear first as a group, then task files as a group; within each group entries are sorted by their logical name (basename in the merged view) in case-sensitive lexicographic order. Physical paths and audience-bucket origin are not considered in sorting — consistent with the merged-view transparency principle from Q1. Tie-breaking is unnecessary because FR-013 makes same-name collisions a hard error.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Operator browses tasks and sees what each one does (Priority: P1)

A human operator runs `taskgate show` at the top of a project and, instead of only seeing task paths, sees a one-line summary next to each task. The output is the **merged view** of what the human audience can run — the union of entries under `.taskgate/shared/` and `.taskgate/human/` — with the audience-bucket directories themselves hidden. Each row's path reflects the task's real physical location, so the operator can tell which bucket a task lives in without the bucket showing up as its own row.

**Why this priority**: This is the everyday discovery flow. Today the old `list` command returned only paths and forced the operator to open files to know what each task did. Adding summaries removes that friction immediately and is the foundation every other story builds on (the same summary string also feeds the directory and AI views). The merged-view model is borrowed from `taskgate run`, which already treats the audience buckets as internal classification rather than user-facing structure.

---

### User Story 2 - Operator inspects a single task in depth (Priority: P2)

The operator has narrowed down to one task and wants the full picture — summary plus the longer body that explains inputs, outputs, side effects, or anything the author wanted to communicate — without leaving the terminal or opening the file.

**Why this priority**: Once summaries exist (P1), the natural next step is "tell me more about this one." Deferring to P2 because the basic browse flow must work first; this story is unblocked the moment summary extraction is in place.

---

### User Story 3 - Operator browses a directory of related tasks (Priority: P2)

The operator runs the show command against a directory (for example, `.taskgate/human/build/`) and sees what that directory is for, followed by a one-line summary for each immediate child entry (whether the child is a task script or a nested directory). Authors can attach a description to a directory by dropping an optional dedicated description file inside it; if no such file is present, the directory still shows cleanly.

**Why this priority**: Tied with Story 2 at P2 because it unlocks navigation by area of concern, which becomes important as soon as a project grows beyond a handful of tasks. Either Story 2 or Story 3 alone delivers value; neither blocks the other.

---

### User Story 4 - AI agent consumes the same view in a machine-friendly form (Priority: P2)

The AI-facing form of the show command emits the same information (path, summary, body, children) but in a shape designed for a program to consume — stable field separators or a structured envelope — so the agent does not have to parse human-oriented formatting. The structure stays the same regardless of whether the agent is showing the root, a directory, or a single file.

**Why this priority**: Same priority as Stories 2 and 3 because the AI consumer is on equal footing with the human consumer in this product. Independent: the AI form can be built from the same extraction logic and exercised through its own command surface.

---

Acceptance scenarios, edge cases, and per-story test recipes for this feature now live as executable tests under `taskgate/testdata/show/*.txtar`, run by `TestShow` in `taskgate/main_test.go`.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST extract a per-entry **summary** (single line) and an optional **body** (multi-line) from a designated annotation block written as comments inside each task file, using one consistent notation across the project.
- **FR-002**: System MUST emit entries with their path even when no annotation is present, so no task is hidden from discovery for lacking documentation.
- **FR-003**: System MUST accept an optional **name argument** identifying either a task or a directory inside the **merged view** (see FR-012). The name is a `run`-style bare entry name (e.g., `build`) or a slash-separated nested name (e.g., `deploy/prod`). Filesystem paths — absolute, cwd-relative, or `.taskgate/`-prefixed — MUST NOT be accepted; supplying one is treated as a "not found" / "invalid name" error. The name resolves against the merged view using the same precedence as `taskgate run` (audience bucket first, then shared) when only one match should win; collisions are handled per FR-013. The command then behaves as:
  - **FR-003a**: When the name resolves to a **task file**, system MUST output that file's real physical path, its summary, and its body (omitting the body section when no body is present).
  - **FR-003b**: When the name resolves to a **directory**, system MUST output that directory's real physical path, its summary, and body (when available), then a one-line entry for each **immediate** child in the merged view (task or sub-directory) showing the child's real path and summary.
  - **FR-003c**: When **no name argument** is given, system MUST present the **merged audience-filtered root view** (see FR-012): the union of immediate entries under the shared bucket and the audience's bucket, with each row's path reflecting the entry's real physical location. The audience-bucket directories MUST NOT appear as rows in this output. The "immediate children only" rule (FR-010) applies to this merged view.
- **FR-004**: System MUST support an **optional** dedicated description file inside any directory under `.taskgate/`. When present, that file supplies the directory's summary and body using the same annotation notation used for task files. When absent, the directory remains addressable; the directory's summary/body section is simply omitted.
- **FR-005**: System MUST expose two distinct output forms — a **human** form via `taskgate show` and a **machine-readable AI** form via `taskgate ai show` — that carry the same underlying information for any given invocation.
- **FR-006**: System MUST produce structured AI output whose shape is uniform across invocations: a single record for a file target, a single record with a children array for a directory target, and a flat collection of records for the no-argument case.
- **FR-007**: System MUST list children of a directory (and the entries of the merged root view) in a stable, deterministic order: **directories first as a group, then task files as a group; within each group, sorted by logical name (basename) in case-sensitive lexicographic order.** Physical path and audience-bucket origin are not part of the sort key. Tie-breaking is unnecessary — FR-013 makes same-name collisions a hard error before the sort is applied.
- **FR-008**: System MUST refuse, with a clear error message, any input that is not a valid `run`-style name within the merged view (per FR-003) — including filesystem paths and any name that resolves outside `.taskgate/`. During directory traversal of resolved entries, system MUST NOT follow symbolic links whose target escapes `.taskgate/`.
- **FR-009**: System MUST treat malformed or partially-present annotations as missing/empty rather than as a fatal error: the invocation succeeds and clearly indicates the gap (human form: omitted section with optional notice; AI form: explicit empty/null field).
- **FR-010**: System MUST NOT recurse beyond a directory's immediate children. Deeper inspection requires the operator (or agent) to issue a follow-up show against the nested path.
- **FR-011**: System MUST treat the dedicated directory description file as the source of the directory's summary/body and MUST NOT also list that file as one of the directory's children, so it does not appear twice.
- **FR-012**: System MUST present a **merged audience-filtered view** that combines entries from the shared bucket (`.taskgate/shared/`) with entries from the audience's bucket (`.taskgate/human/` for `taskgate show`; `.taskgate/ai/` for `taskgate ai show`). The audience-bucket directories themselves (`human/`, `ai/`, `shared/`) MUST NOT appear as entries in any show output. Every output row's path field MUST reflect the entry's real physical location under `.taskgate/`. This model is intentionally aligned with the existing `taskgate run` / `taskgate ai run` task-resolution behavior.
- **FR-013**: System MUST detect **name collisions** in the merged view — i.e., the same logical name appearing in both the audience bucket and the shared bucket (whether as a task file, a directory, or a mix). On detecting any collision in the region being shown (no-argument browse, explicit name reference, or the children listing of a directory target), system MUST emit a warning naming all conflicting real paths and exit with a **non-zero status without emitting partial output of the conflicting region**. The human form emits the warning on stderr; the AI form emits a structured error record (final shape deferred to plan phase, consistent with FR-006). Collisions outside the region being shown do not block the invocation. This makes show stricter than `taskgate run`, which silently resolves collisions via audience-first precedence — show treats them as an authoring error to fix.

Vocabulary used above (Task entry, Audience bucket, Merged view, Annotation block, etc.) is defined in [`docs/glossary.md`](../../docs/glossary.md).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: An operator running `taskgate show` with no arguments in a project where every task has a summary annotation can identify the intent of every listed task without opening any file — measured by asking a fresh reader to match each listed task to a one-sentence description of its purpose and reaching 100% match on annotated tasks.
- **SC-002**: An AI agent can parse 100% of `taskgate ai show` invocations (no path, file path, directory path) using a single parser implementation, with no special-case handling per invocation shape.
- **SC-003**: For any directory under `.taskgate/`, a single show invocation produces the directory's intent plus a one-row-per-child summary view — operator goes from "I want to know what's in this folder" to having the answer in one keystroke.
- **SC-004**: Zero tasks are dropped from the show output for the sole reason of missing or malformed annotations. Every task file visible to the audience (i.e., reachable in the merged view per FR-012) appears at the appropriate level.
- **SC-005**: Adding a directory description file to one directory has no effect on the show output for any other directory — verified by snapshotting other directories' output before and after.

## Assumptions

- The **comment notation** used for the annotation block is a single project-wide convention chosen at design time; the spec does not pin a specific syntax. The plan phase will lock the exact markers (e.g., a header line followed by a body region) once the constraints of the supported task script formats are reviewed.
- The **directory description filename** is a single reserved name agreed at design time, chosen so it cannot collide with a legitimate task script name. The spec does not pin the exact name.
- The **concrete wire format** of `taskgate ai show` (envelope JSON, NDJSON, etc.) is intentionally deferred to the plan phase. The spec only commits to the contract in FR-006: a structure parseable by a single parser implementation across the three invocation shapes (no-argument list, file target, directory target).
- "Inside `.taskgate/`" means within the `.taskgate/` directory of the current project, located as the existing project-root detection logic already locates it (no change to that logic).
- The current `taskgate list` and `taskgate ai list` subcommands are **retired and replaced** by `taskgate show` and `taskgate ai show`. The old subcommand names are not kept as aliases; callers must migrate. This is by explicit user decision — see the Naming note at the top.
- The merged-namespace model (FR-012) is **intentionally aligned with `taskgate run` / `taskgate ai run`**, which already resolve a bare task name across `shared/` + the audience's bucket. Show is the "describe" counterpart to run's "execute"; the user-facing namespace is identical.
- The user-facing input surface is **names only** (`run`-style bare or slash-separated). Filesystem paths are never accepted as input, even though output `path` fields preserve real physical locations. This intentionally hides the audience-bucket layer from operators and AI clients, and removes any "filesystem-path-vs-name" interpretation ambiguity at the input boundary.
- The human form (`taskgate show`) is permitted to format output for human reading (alignment, labels, etc.); machine consumers should use `taskgate ai show`. Because the old `list` is being retired with no compatibility shim, no constraint is imposed to keep the human form line-grep-friendly.
- The body, when present, is rendered as-is (verbatim text from the annotation) in the human form; no transformation or markdown rendering is in scope for v1.
- "Children" of a directory means **immediate** children only. Recursive tree views are out of scope for v1.
- Performance targets are inherited from the existing list command's footprint: latency stays in the "instant" range (sub-second) for projects with a typical number of tasks (tens to low hundreds).
