# Feature Specification: Show Subcommand — Tasks and Directories with Descriptions

## User Scenarios

### User Story 1 - Operator browses tasks and sees what each one does

A human operator runs `taskgate show` at the top of a project and, instead of only seeing task paths, sees a one-line summary next to each task. The output is the **merged view** of what the human audience can run — the union of entries under `.taskgate/shared/` and `.taskgate/human/` — with the audience-bucket directories themselves hidden. Each row's path reflects the task's real physical location, so the operator can tell which bucket a task lives in without the bucket showing up as its own row.

---

### User Story 2 - Operator inspects a single task in depth

The operator has narrowed down to one task and wants the full picture — summary plus the longer body that explains inputs, outputs, side effects, or anything the author wanted to communicate — without leaving the terminal or opening the file.

---

### User Story 3 - Operator browses a directory of related tasks

The operator runs the show command against a directory (for example, `.taskgate/human/build/`) and sees what that directory is for, followed by a one-line summary for each immediate child entry (whether the child is a task script or a nested directory). Authors can attach a description to a directory by dropping an optional dedicated description file inside it; if no such file is present, the directory still shows cleanly.

---

### User Story 4 - AI agent consumes the same view in a machine-friendly form

The AI-facing form of the show command emits the same information (path, summary, body, children) but in a shape designed for a program to consume — stable field separators or a structured envelope — so the agent does not have to parse human-oriented formatting. The structure stays the same regardless of whether the agent is showing the root, a directory, or a single file.

---

## Related

- **Functional requirements**: [`docs/show/requirements.md`](../../docs/show/requirements.md)
- **Acceptance scenarios** (executable tests): [`tests/features/show/*.feature`](../../tests/features/show) — run via `uv run pytest` or `taskgate run e2e`
- **Design decisions** (ADRs): [`docs/show/adr/`](../../docs/show/adr)
- **Vocabulary**: [`docs/show/glossary.md`](../../docs/show/glossary.md)

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: An operator running `taskgate show` with no arguments in a project where every task has a summary annotation can identify the intent of every listed task without opening any file — measured by asking a fresh reader to match each listed task to a one-sentence description of its purpose and reaching 100% match on annotated tasks.
- **SC-002**: An AI agent can parse 100% of `taskgate ai show` invocations (no path, file path, directory path) using a single parser implementation, with no special-case handling per invocation shape.
- **SC-003**: For any directory under `.taskgate/`, a single show invocation produces the directory's intent plus a one-row-per-child summary view — operator goes from "I want to know what's in this folder" to having the answer in one keystroke.
- **SC-004**: Zero tasks are dropped from the show output for the sole reason of missing or malformed annotations. Every task file visible to the audience (i.e., reachable in the merged view per FR-012) appears at the appropriate level.
- **SC-005**: Adding a directory description file to one directory has no effect on the show output for any other directory — verified by snapshotting other directories' output before and after.

## Assumptions

- "Inside `.taskgate/`" means within the `.taskgate/` directory of the current project, located as the existing project-root detection logic already locates it (no change to that logic).
- The merged-namespace model (FR-012) is **intentionally aligned with `taskgate run` / `taskgate ai run`**, which already resolve a bare task name across `shared/` + the audience's bucket. Show is the "describe" counterpart to run's "execute"; the user-facing namespace is identical.
- The user-facing input surface is **names only** (`run`-style bare or slash-separated). Filesystem paths are never accepted as input, even though output `path` fields preserve real physical locations. This intentionally hides the audience-bucket layer from operators and AI clients, and removes any "filesystem-path-vs-name" interpretation ambiguity at the input boundary.
- The human form (`taskgate show`) is permitted to format output for human reading (alignment, labels, etc.); machine consumers should use `taskgate ai show`. No constraint is imposed to keep the human form line-grep-friendly.
- The body, when present, is rendered as-is (verbatim text from the annotation) in the human form; no transformation or markdown rendering is in scope for v1.
- Performance target: latency stays in the "instant" range (sub-second) for projects with a typical number of tasks (tens to low hundreds).
