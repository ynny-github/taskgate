---
name: persisting-feature-specs
description: Use when a planning-phase feature spec has been implemented and its prose sections (acceptance scenarios, edge cases, design rationale, vocabulary, requirements) need to be relocated into single-source-of-truth artifacts before drift sets in
---

# Persisting feature specs

## Overview

A planning-phase spec is **temporary**: it exists to get the team to "we agree on what to build". Once code ships, the spec keeps re-declaring things that now live in tests, ADRs, and code — every duplicate is a drift source. **Persisting** the spec means moving each section to the single artifact type that owns it, so the spec stops competing with the things it spawned.

**Core principle:** each artifact type has exactly one canonical home. After persistence, `spec.md` is a thin index of what stays declarative (user intent, quality bars, deliberate scope decisions).

## When to Use

- Feature implementation has merged or is about to merge — the spec is no longer "what we plan", it's "what we built".
- `spec.md` is duplicating content that tests, code comments, or contracts also assert.
- The spec has speckit/template-phase header noise (`Status: Draft`, Clarifications log, "Why this priority") that no longer earns its keep.
- Cross-cutting decisions live in three places (spec body, contracts/, research.md) with the same words.

**Don't use mid-implementation** — the spec is still the working artifact.
**Don't use** if the spec is the deliverable for external stakeholders (regulatory, contractual).

## Mapping: spec section → canonical home

| In `spec.md` | Goes to | Why |
|---|---|---|
| User Story narrative | **stays in spec.md** | The "what & why" for human readers |
| `(Priority: P1/P2)` / `**Why this priority**` | **delete** | Implementation sequencing — done now |
| Acceptance Scenarios (Given/When/Then) | **`tests/features/<feature>/*.feature`** (or other executable test layer) | Verification belongs in code, not prose |
| Independent Test recipes | **delete** | Replaced by the executable suite |
| Edge Cases | **`tests/features/<feature>/*.feature`** | Same |
| Functional Requirements (FR-XXX) | **`docs/<feature>/requirements.md`** | Normative declarations, not feature-spec-folder content |
| Key Entities / vocabulary | **`docs/<feature>/glossary.md`** (or `docs/glossary.md` if cross-feature) | One canonical definition |
| Success Criteria | **stays in spec.md** | Aspirational quality bars, not test scenarios |
| Assumptions | **slim**: keep load-bearing scope/policy decisions, delete those now covered by ADRs | Deliberate scope; trim what an ADR replaces |
| Clarifications Q&A log | **delete** | Each Q is now embodied in an FR or ADR; the log itself is drift surface |
| Naming note / `Status: Draft` / `Feature Branch` / verbatim user prompt | **delete** | Planning-phase header noise; belongs in PR description or git log |

Separate files in `specs/<feature>/`:

| File | Disposition |
|---|---|
| `research.md` (design rationale + alternatives) | Promote each decision to `docs/<feature>/adr/000X-*.md` (Nygard format); delete |
| `contracts/*.md` | Per-scenario rules → test file headers. Cross-cutting policy → `tests/features/<feature>/README.md`. Delete the contracts/ dir. |
| `quickstart.md` (manual test recipe) | Delete — the executable test suite replaces it |
| `plan.md` | Keep as implementation history, or delete if not consulted |

## Target layout (per-feature scoping)

```
specs/<feature>/
└── spec.md                    # slim: User Stories, Related, SC, Assumptions

docs/<feature>/
├── requirements.md            # FR-XXX (normative)
├── glossary.md                # vocabulary
└── adr/
    ├── README.md
    └── 000X-*.md              # design decisions, Nygard format

tests/features/<feature>/
└── *.feature                  # executable acceptance + edge cases
```

`spec.md` gains a `## Related` block pointing at the four artifacts above. Future features (e.g. `<other-cmd>`) get parallel `docs/<other-cmd>/`, `tests/features/<other-cmd>/` siblings.

## Process (incremental commits, in this order)

1. **Convert verification first.** Acceptance Scenarios + Edge Cases → executable tests. This is the irreversible win; once tests exist, the prose is provably redundant. Don't delete the prose until the tests are green.
2. **Promote design rationale to ADRs.** Each "we picked X over Y because…" in `research.md` (or scattered in the spec) becomes a numbered ADR with **Context / Decision / Consequences / Alternatives considered**. Delete `research.md` only after the ADRs land.
3. **Extract glossary.** Terms used across the spec, tests, and ADRs deserve one definition.
4. **Move requirements out of `spec.md`.** FRs are project-level normative content; `docs/<feature>/requirements.md` is their home. The FR-XXX identifiers stay stable so tests / ADRs that reference them still work.
5. **Delete drift-prone planning artifacts.** Clarifications log, priority labels, contracts/ that duplicate tests, quickstart, header meta.
6. **Add `## Related` to `spec.md`** as the navigation hub.
7. **Per-feature scoping.** Once a second feature appears (or in anticipation), move show-specific docs/ and tests/ under `<feature>/` subdirs. Truly cross-feature items (e.g. an ADR consumed by multiple commands) get promoted back to top-level when that day comes.

Make each step its own commit. Reviewers can sanity-check one move at a time, and `git revert` works cleanly if any step over-shoots.

## Common mistakes

- **Deleting acceptance scenarios before writing the tests.** Drops the verification. Always: tests green → then delete prose.
- **Keeping both the FR list and the test scenarios as "spec".** Both will drift. Pick one canonical: FRs as declarations + tests as samples (current convention) OR tests as the only spec (cucumber-style). The middle breeds inconsistency.
- **Putting ADRs under `specs/<feature>/`.** ADRs outlive features. They belong at `docs/<feature>/adr/` or `docs/adr/`, not in the planning-phase folder.
- **Forgetting the `## Related` block.** Readers can't find the now-distributed artifacts.
- **Trying to persist everything in one commit.** Hard to review, hard to revert. Use the 7 ordered steps as 7 commits (or close to it).
- **Treating `Clarifications` as audit log worth preserving.** `git log` is the audit log. The Clarifications section will rot out of sync within weeks.

## Red flags

- "The acceptance criteria prose is still useful as documentation." → That's drift waiting to happen. The Gherkin scenario name + steps IS the documentation.
- "Let me keep the FR list in `spec.md` AND in `docs/requirements.md`." → They will diverge in two releases.
- "We can't express this scenario as an executable test." → At what level (unit / integration / e2e) **could** you? Find that level and write the test there.
- "I'll persist after the next milestone." → "Later" means never; each new commit accretes contradictions against frozen prose.
- "Status: Draft is part of the speckit template, keep it." → Templates are scaffolding. Remove the scaffolding once the building is up.

## Verification

After persisting, sanity-check:

```
git grep "FR-001" -- specs docs tests
```
Each FR should appear in `docs/<feature>/requirements.md` (declaration) and `tests/features/<feature>/*.feature` (verification), NOT in `spec.md`.

`spec.md` should be < 100 lines and read as "user-facing intent + Related + SC + Assumptions". If it's longer, you've left planning-phase residue.

The full test suite still passes — verification was preserved, only its representation changed.
