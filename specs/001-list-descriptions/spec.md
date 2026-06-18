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

**Independent Test**: Place several task scripts split across `.taskgate/human/` and `.taskgate/shared/`, give some a summary annotation and leave others bare, run `taskgate show` with no path argument, and verify (a) every task from both buckets appears as its own row with its real path, (b) annotated tasks show their summary, (c) bare tasks still appear with path only, and (d) `human/` and `shared/` themselves never appear as rows.

**Acceptance Scenarios**:

1. **Given** a project with `.taskgate/human/build` and `.taskgate/shared/lint`, each carrying a summary annotation, **When** the operator runs `taskgate show` with no path argument, **Then** the output lists both tasks with their real paths (`.taskgate/human/build`, `.taskgate/shared/lint`) and their summaries on the same rows, and the output does **not** contain rows for `human/`, `shared/`, or `ai/`.
2. **Given** a task script that has no annotation block, **When** the operator runs `taskgate show` with no path argument, **Then** that task still appears in the merged view using its real path, and no error is raised.
3. **Given** a task `build` that exists in both `.taskgate/human/` and `.taskgate/shared/` (a name collision in the merged view), **When** the operator runs `taskgate show` with no path argument, **Then** the command emits a collision warning listing the conflicting real paths (e.g., "name `build` collides: `.taskgate/human/build` vs `.taskgate/shared/build`") and exits with a non-zero status; no merged listing is printed.
4. **Given** the same project state, **When** the AI counterpart runs `taskgate ai show` with no path argument, **Then** the output is the merged view of `.taskgate/shared/` ∪ `.taskgate/ai/` (NOT `human/`), confirming that the audience filter is `audience ∪ shared`.

---

### User Story 2 - Operator inspects a single task in depth (Priority: P2)

The operator has narrowed down to one task and wants the full picture — summary plus the longer body that explains inputs, outputs, side effects, or anything the author wanted to communicate — without leaving the terminal or opening the file.

**Why this priority**: Once summaries exist (P1), the natural next step is "tell me more about this one." Deferring to P2 because the basic browse flow must work first; this story is unblocked the moment summary extraction is in place.

**Independent Test**: Author a task with both a summary line and a multi-paragraph body in its annotation, run the show command with that task's **name** (run-style, no path) as the argument, and verify both the summary and the full body are printed.

**Acceptance Scenarios**:

1. **Given** a task file with a summary and a body in its annotation, **When** the operator runs `taskgate show <task-name>`, **Then** the output prints the resolved entry's real physical path, the summary, and the full body in that order.
2. **Given** a task file with a summary but no body, **When** the operator runs `taskgate show <task-name>`, **Then** the output prints the path and the summary; the body section is omitted rather than shown as empty.
3. **Given** a name that does not resolve to any task or directory in the merged view (whether genuinely absent, or because the operator passed a filesystem path like `.taskgate/human/build` or `/abs/path` instead of a bare name), **When** the operator runs `taskgate show` with it, **Then** the command exits with a clear error stating show accepts only `run`-style names (bare or slash-separated), and listing the audience scope that was searched.

---

### User Story 3 - Operator browses a directory of related tasks (Priority: P2)

The operator runs the show command against a directory (for example, `.taskgate/human/build/`) and sees what that directory is for, followed by a one-line summary for each immediate child entry (whether the child is a task script or a nested directory). Authors can attach a description to a directory by dropping an optional dedicated description file inside it; if no such file is present, the directory still shows cleanly.

**Why this priority**: Tied with Story 2 at P2 because it unlocks navigation by area of concern, which becomes important as soon as a project grows beyond a handful of tasks. Either Story 2 or Story 3 alone delivers value; neither blocks the other.

**Independent Test**: Create a sub-directory under one of the audience buckets containing a mix of task files and one nested sub-directory, with and without a dedicated description file at its root; run the show command with the sub-directory's **name** (run-style, e.g., `taskgate show deploy`); verify the directory's own summary + body is printed when the description file exists, only its real path is printed when it does not, and each immediate child's summary appears on its own row.

**Acceptance Scenarios**:

1. **Given** a directory `.taskgate/human/deploy/` that contains a dedicated description file (with summary and body) and three child task files each with their own summary annotation, **When** the operator runs `taskgate show deploy`, **Then** the output prints the directory's real path, summary, and body, followed by one row per immediate child showing each child's real path and summary.
2. **Given** the same directory but without a dedicated description file, **When** the operator runs `taskgate show deploy`, **Then** the output prints the directory's real path with no summary/body section, followed by one row per immediate child showing each child's real path and summary.
3. **Given** a directory `.taskgate/human/deploy/` containing a nested sub-directory `prod/`, **When** the operator runs `taskgate show deploy`, **Then** `prod/` appears in the children list with its summary (taken from its own description file if present, omitted otherwise); the listing does not recurse beyond the immediate children. To go deeper, the operator runs `taskgate show deploy/prod`.
4. **Given** a directory whose dedicated description file is malformed (cannot be parsed for a summary), **When** the operator runs `taskgate show <name>` against it, **Then** the listing still succeeds, the directory's summary is omitted, and a non-fatal notice surfaces in the human view (and is silent in the AI view).

---

### User Story 4 - AI agent consumes the same view in a machine-friendly form (Priority: P2)

The AI-facing form of the show command emits the same information (path, summary, body, children) but in a shape designed for a program to consume — stable field separators or a structured envelope — so the agent does not have to parse human-oriented formatting. The structure stays the same regardless of whether the agent is showing the root, a directory, or a single file.

**Why this priority**: Same priority as Stories 2 and 3 because the AI consumer is on equal footing with the human consumer in this product. Independent: the AI form can be built from the same extraction logic and exercised through its own command surface.

**Independent Test**: Run `taskgate ai show` (a) with no argument, (b) with a task name, and (c) with a directory name, and verify each invocation produces output that can be parsed by a single small parser into records with explicit `path`, `summary`, `body`, and `children` fields.

**Acceptance Scenarios**:

1. **Given** the same project state used in Story 1, **When** an AI agent invokes `taskgate ai show` with no argument, **Then** the output is structured (not formatted prose) and contains one record per merged-view entry with explicit `path` (real physical path) and `summary` fields.
2. **Given** the task-name invocation from Story 2, **When** an AI agent invokes `taskgate ai show <task-name>`, **Then** the output is a single structured record containing `path`, `summary`, and `body`.
3. **Given** the directory invocation from Story 3, **When** an AI agent invokes `taskgate ai show <directory-name>`, **Then** the output is a single structured record containing the directory's `path`, `summary`, `body`, and a `children` array, where each child has at least `path` and `summary`.
4. **Given** a task with no summary annotation, **When** an AI agent invokes either show form against it, **Then** the structured output still includes the record with an explicit empty/null summary marker rather than dropping it.

---

### Edge Cases

- A task file exists but is unreadable (e.g., permission denied): the output surfaces the path with a clear notice and continues with the remaining entries rather than aborting the whole invocation.
- The annotation block exists but contains only whitespace after the summary marker: the summary is treated as empty (same handling as "no summary present").
- Other comment lines appear between the shebang and the annotation envelope (e.g., a `shellcheck disable` pragma, a copyright header, or any normal narrative comment): the recognizer scans past them to find the opening `---` delimiter; their content is not part of the parsed annotation. Lines *inside* the envelope are parsed as YAML in full — there is no concept of "annotation lines mixed with normal comments" within the envelope itself.
- A directory's dedicated description file is itself a runnable task file: the file's annotation is used as the directory's summary/body, and the file is **not** listed again as one of the directory's children (avoids double-counting).
- The user supplies what looks like a filesystem path (absolute, cwd-relative, or `.taskgate/`-prefixed): rejected with a clear error explaining show accepts only `run`-style bare or slash-separated names, never filesystem paths. This applies even if the path would resolve to a real entry under `.taskgate/`.
- A symbolic link inside `.taskgate/` points to a target outside `.taskgate/`: the show command refuses to follow the link during directory traversal; it shows the link as an entry but does not read or display the off-workspace target's body.
- A directory contains hundreds of children: output stays usable (no truncation without warning); the human form must remain readable and the AI form must remain parseable.
- The same logical name exists in **both** the audience bucket and the shared bucket (e.g., `.taskgate/human/build` and `.taskgate/shared/build` both present, as files or directories or one of each): treated as a **hard error** per FR-013. The show command does not allow such collisions and exits with a non-zero status after emitting a warning that lists all conflicting real paths. This applies whether the conflict surfaces in the no-argument browse, in an explicit `taskgate show <name>` reference, or while listing the children of a directory target. Operators resolve the collision at the source — by renaming or removing one of the entries — before show can produce output for the affected region.
- The user supplies a name that does not resolve to any entry in the merged view: show exits with a clear "not found" error listing the audience scope that was searched (e.g., "`build` not found in `.taskgate/human/` or `.taskgate/shared/`").

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST extract a per-entry **summary** (single line) and an optional **body** (multi-line) from a designated annotation block written as comments inside each task file, using one consistent notation across the project.
- **FR-002**: System MUST emit entries with their path even when no annotation is present, so no task is hidden from discovery for lacking documentation.
- **FR-003**: System MUST accept an optional **name argument** identifying either a task or a directory inside the **merged view** (see FR-012). The name is a `run`-style bare entry name (e.g., `build`) or a slash-separated nested name (e.g., `deploy/prod`). Filesystem paths — absolute, cwd-relative, or `.taskgate/`-prefixed — MUST NOT be accepted; supplying one is treated as a "not found" / "invalid name" error. The name resolves against the merged view using the same precedence as `taskgate run` (audience bucket first, then shared) when only one match should win; for the no-argument browse case both rows of a collision are shown (see Story 1 scenario 3). The command then behaves as:
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

### Key Entities *(include if feature involves data)*

- **Task entry**: A single executable file under `.taskgate/{human,ai,shared}/...`. Has a path; may carry an annotation block providing a summary and an optional body. A task's physical location determines which audience(s) can see it via show.
- **Directory entry**: A folder under `.taskgate/`. Has a path; may carry a summary and body via an optional dedicated description file placed inside it; lists its immediate children (which are themselves task entries or directory entries).
- **Audience bucket**: One of the three reserved top-level directories under `.taskgate/`: `human/`, `ai/`, `shared/`. Acts as an internal audience classifier and is **invisible** in show output — never appears as its own row. Tasks under `shared/` are visible to both audiences; tasks under `human/` are visible only to `taskgate show`; tasks under `ai/` are visible only to `taskgate ai show`.
- **Merged view**: The user-facing tree that show exposes. For `taskgate show` it is the union of entries directly under `.taskgate/shared/` and `.taskgate/human/`; for `taskgate ai show` it is the union of `.taskgate/shared/` and `.taskgate/ai/`. The buckets themselves are folded out; entry paths retain their real physical location.
- **Annotation block**: The summary + optional body, expressed in the project's chosen comment notation, embedded at the top of a task file or in a directory's description file. Two parts: a single-line **summary** and a free-form multi-line **body**.
- **Directory description file**: An optional file inside a directory that carries that directory's annotation block. Identified by a project-wide reserved name. Its presence is never required; its absence is not an error.
- **Output record**: The unit of output. Carries the entry's real physical path, its summary (possibly empty), its body (only for the single-target file/directory case), and — for a directory target — the list of immediate child records (path + summary only) in the merged view.
- **Audience mode**: The output shape and the filter applied. **Human** mode (`taskgate show`) emits formatted text and merges `shared/` ∪ `human/`. **AI** mode (`taskgate ai show`) emits a structured form and merges `shared/` ∪ `ai/`. Both carry the same information for any given invocation.

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
