# Architecture Decision Records

ADRs in this directory capture significant architectural decisions and the rationale for them. They are reference material for future revisits (RFCs, refactors, format proposals), not day-to-day implementation guidance.

Format: lightly adapted from Michael Nygard's ADR template. Each ADR has a Status, Context, Decision, Consequences, and Alternatives considered.

| # | Title | Status |
|---|---|---|
| 0001 | [Annotation format for task descriptions](0001-annotation-format.md) | Accepted |
| 0002 | [Directory description filename](0002-directory-description-filename.md) | Accepted |
| 0003 | [AI output wire format](0003-ai-output-wire-format.md) | Accepted |

These ADRs were extracted from the original `specs/001-list-descriptions/research.md` written during the feature's plan phase. The implementation under `taskgate/`, the E2E suite (`taskgate/testdata/show/*.txtar`), and the CLI behaviour are the canonical source of truth; the ADRs record **why** the implementation looks the way it does, including alternatives that were considered and rejected.
