# Phase 0 Research: Show Subcommand

**Feature**: Show Subcommand — Tasks and Directories with Descriptions
**Date**: 2026-06-16

Resolves the three items the spec deferred to the plan phase (annotation notation, directory description filename, AI wire format), plus one project-hygiene item (retirement of the legacy `list` subcommand).

---

## R1: Annotation comment notation

**Decision**: **YAML front-matter** inside a leading comment block, delimited by `---` lines at both ends. The delimiters and the lines inside the envelope are each prefixed with a **line-comment prefix appropriate to the script's language**, so the script stays syntactically valid in its own language. Marker is intentionally **bare** (`---`, not `---taskgate`) to keep the convention portable across tools.

**Supported line-comment prefixes** (the parser tries them in this order; the first that matches determines the prefix used throughout the envelope of that file):

| Prefix | Languages |
|---|---|
| `#` | sh, bash, zsh, python, ruby, perl, makefile, yaml |
| `//` | JavaScript, TypeScript, Go (as `go run` script), Java, Rust, Swift, C-family |
| `--` | Lua, SQL, Haskell, Ada |
| `;` | Lisp / Clojure / Scheme, INI-style |
| (none) | `_index` files written as bare YAML, no comment prefix |

Block-style comments (`/* ... */`, `<!-- ... -->`, etc.) are out of scope: the recognizer handles line-comment styles only.

Recognizer:

- Scan the head of the file. Skip an optional `#!`-shebang first line (regardless of which comment prefix the body uses — `#!` is always the shebang).
- For each remaining line until the first match, test against every supported prefix. A line **opens the envelope** if, after stripping the candidate prefix (with or without one trailing space) and trimming trailing whitespace, the remaining content equals exactly `---`. The prefix that matched is now "the prefix" for this file.
- Continue scanning for the **closing** line: a line that matches the same form with the same prefix. Different prefixes do not close the envelope.
- The match MUST be strict equality on the content after prefix stripping. Lines like `# --- foo` or `// ----` do not match.
- Strip the same prefix (with or without one trailing space) from every line **inside** the block. Parse the resulting buffer as YAML.

Recognized top-level keys (inside the YAML):

| Key | Type | Notes |
|---|---|---|
| `summary` | string | One line; defines the entry's summary. |
| `body` | string | Often multi-line; use YAML's literal-block scalar (`body: |`) for multi-line content. |

All other top-level YAML keys are silently ignored (forward-compatibility: future fields like `tags`, `since`, `author` drop in without a recognizer change).

Example task files (one per supported prefix):

```sh
#!/bin/sh
# ---
# summary: Build the project for the current platform.
# body: |
#   Reads VERSION from the environment.
#   Exits non-zero on build failure.
# ---

set -euo pipefail
go build -o bin/taskgate ./taskgate
```

```js
#!/usr/bin/env node
// ---
// summary: Run the dev server.
// body: |
//   Restarts on file changes.
// ---

require('./server').start();
```

```lua
#!/usr/bin/env lua
-- ---
-- summary: Format the project.
-- body: |
--   Skips vendor/.
-- ---

require('formatter').run()
```

```
; ---
; summary: Initialize the Lisp environment.
; ---

(load "init.lisp")
```

Example `_index` (directory description file — no shebang, not executable). Any of the supported prefixes work; bare YAML (no prefix) is also accepted, since `_index` is not executed as a script:

```text
# ---
# summary: Promote a build to an environment.
# body: |
#   Each child task corresponds to a deploy target. Children run idempotently.
# ---
```

```text
---
summary: Promote a build to an environment.
body: |
  Each child task corresponds to a deploy target. Children run idempotently.
---
```

**Why bare `---`** (no tool-name suffix):

- **Portability across tools**: `---` ... `---` is the standard YAML front-matter delimiter used by Hugo, Jekyll, Pandoc, MkDocs, Pelican, etc. Annotations written in this form can be parsed by any of those tools (or future ones) by stripping the line-comment prefix as a pre-pass.
- **Visual cleanliness**: dashes alone read as "front-matter boundary" with no decoration noise.
- **No tool-name coupling**: if the project ever renames or forks the binary, the on-disk format does not need to migrate.

**Why multiple comment prefixes** (not `#` only):

- Task scripts in `.taskgate/{human,ai,shared}/` may be written in any language whose runtime can be invoked from a shebang (or that is executed via an explicit interpreter). Restricting annotations to `#`-style would exclude Node.js / JavaScript, TypeScript via `tsx` / `deno`, Lua, Lisp, etc.
- The recognizer auto-detects the prefix per file, so authors do not need to declare anything. The first delimiter line is the source of truth.
- Mixing prefixes inside a single envelope is not supported and not needed: each file is in one language and uses one comment style.

**Known limitation** (acknowledged, not eliminated):

A `<prefix> ---` line (e.g., `# ---`, `// ---`, `-- ---`) that appears inside a YAML literal-block `body: |` but has been written at the wrong indentation level — aligned with the delimiter rather than indented under `body:` — will close the envelope early. Two layers of mitigation reduce the realistic incidence to near zero:

1. The recognizer demands **exact string equality** on the delimiter content (`---` and nothing else after the prefix is stripped). A body line such as `#   ---` (two-space indent under the prefix, which is what YAML requires for a `body: |` continuation) strips to `  ---`, which has leading whitespace and therefore does **not** match. Authors and AIs who follow normal YAML indentation rules are safe by construction.
2. YAML itself rejects a column-0 `---` inside what would otherwise be a literal-block — it interprets that as a new document boundary, not body content. So even an attempt to write the offending line typically fails YAML parsing, which downgrades to FR-009 (annotation treated as empty, non-fatal).

Authors who genuinely need to embed the literal text `---` on its own line in a body should either indent it properly under `body: |` (which the recognizer treats as content) or use a quoted scalar.

**Rationale**:

- **AI-friendly authoring**: YAML is in every AI assistant's wheelhouse; the `body: |` literal-block scalar keeps multi-line content readable without escape acrobatics. Generation rarely fails on syntax.
- **Extensible**: new top-level fields drop in without changing the recognizer (`tags`, `since`, `author`, `deprecated`, etc.) — important once docs/tooling grow.
- **Preserves the script as valid source in its own language**: every line in the block carries the script's line-comment prefix (`#` for shell/python, `//` for JavaScript/Go, `--` for Lua, `;` for Lisp), so the kernel still respects the shebang and language-specific comment-aware tools (shellcheck, eslint, luacheck, etc.) are unaffected.
- **Structured boundaries**: the `<prefix> ---` delimiters (e.g., `# ---`, `// ---`, `-- ---`, `; ---`) make "where does the annotation end" unambiguous to humans, AIs, and the parser. No "is this comment line a continuation or unrelated?" ambiguity.

**Implementation cost**: pulls in `gopkg.in/yaml.v3` as a new module dependency (see plan.md Technical Context — Primary Dependencies). Trade-off accepted: one well-maintained, low-risk YAML parser vs. a fragile hand-rolled subset.

**Alternatives considered**:

- **Tag-based `#: ` / `#:: ` markers**: more compact, no YAML dependency, but multi-line `body` continuation is brittle when an AI writes it (must remember to prefix every continuation line, easy to forget); not extensible without inventing more markers. The previous draft of this research note picked this option; superseded.
- **Implicit "first comment block is the description"**: zero syntax, but every leading shellcheck pragma or copyright header is silently consumed as the description. No way to write an ordinary leading comment without it leaking into output.
- **Tag-based `# @summary:` / `# @body:` (Doxygen-style)**: familiar but verbose; body continuation rules vary across communities.
- **Sidecar Markdown file (`<name>.md` next to the script)**: cleanest separation but doubles file count and adds script/sidecar drift risk; rejected in favor of in-file front-matter to keep each task self-contained.

**Edge handling** (driven by FR-009):

- Missing `summary` key: treated as no summary (rendered empty / null per audience-form rules).
- Missing `body` key: body section omitted from the output.
- YAML parse error inside the block (malformed indentation, syntax error): the annotation is treated as missing entirely; the entry still lists with empty summary and no body. The human form emits a non-fatal notice on stderr; the AI form is silent (per spec edge case).
- Empty value (`summary: ""` or `summary:` with nothing after): treated as empty string, equivalent to "no summary present".
- Unknown YAML keys (`tags: [...]`): silently ignored.
- Multiple envelopes in the file: only the first complete (open + close) one is recognized; subsequent occurrences are left untouched.
- Mismatched prefix between opener and closer (e.g., opener `# ---`, closer `// ---`): closer is not recognized, the envelope is considered unclosed, and the annotation is treated as missing. Authors should keep one prefix per file (which mirrors using one language per script file).
- The script body uses one comment style for code (e.g., `//` in JavaScript) but the envelope is written with a different prefix (e.g., `# ---`): the script will fail at runtime in its host language because `# ---` is not a valid JS comment. This is a script-authoring error, not a parser issue.
- A delimiter-matching line that an author intends as body content but writes without YAML's required body indentation: see the "Known limitation" subsection above for the two-layer mitigation. In normal authoring this does not occur because the YAML literal-block scalar requires indented continuation lines.

---

## R2: Directory description filename

**Decision**: `_index` (no extension), located directly inside the directory it describes.

- A file literally named `_index` inside any directory under `.taskgate/` is interpreted as that directory's annotation source.
- It uses the same YAML front-matter notation defined in R1: a `<prefix> ---` ... `<prefix> ---` envelope around YAML with `summary` / `body` keys, where `<prefix>` is any of the supported line-comment prefixes (`#`, `//`, `--`, `;`). The prefix may also be omitted entirely in `_index` (since it isn't executed as a script), and the recognizer accepts the bare `---` form as well.
- The file is **not** required to be executable. If it happens to be executable, it is still consumed only as a description source (and therefore excluded from the merged-view children listing, per FR-011).

**Rationale**:

- Cannot collide with a legitimate task script name because `taskgate run` resolves user-supplied bare names against actual file basenames; the leading underscore makes `_index` look obviously not-a-task to operators and discourages accidental `taskgate run _index` attempts.
- Hugo-style precedent: `_index` is widely understood as "this folder, not its children." Familiar to most.
- Sorts before any alphabetic basename, which is irrelevant given FR-007's "directories first, then files" grouping (the description file is never listed as a child anyway).
- No extension keeps the file evidently a tooling artifact rather than a document; reduces tooling confusion (e.g., editors won't auto-render it as markdown).

**Alternatives considered**:

- `README` / `README.md`: too overloaded. Many repos already use READMEs for human documentation that doesn't follow this annotation format. Forcing collision is too constraining.
- `.description` / `.taskgate.md`: a dotfile hides from `ls` by default, which slightly works against discoverability for new contributors.
- `_taskgate-description`: explicit but verbose, and authors must type it more often than they'd like.

**Edge handling**:

- `_index` is itself never listed as a child entry of its containing directory (FR-011/FR-012).
- If `_index` exists but is empty, has no `---` envelope (with or without a comment prefix), or contains malformed YAML, the directory's summary and body are treated as empty per FR-009; the directory still lists.
- `_index` in `.taskgate/human/` or `.taskgate/shared/` or `.taskgate/ai/` itself: not meaningful, because audience-bucket directories never surface as user-facing entries. If present, ignored by show (with no warning); operators can still place one for narrative purposes but it has no effect on output.

---

## R3: AI output wire format

**Decision**: Single-document **envelope JSON** with a top-level `"kind"` discriminator. One JSON document per `taskgate ai show` invocation, on stdout, terminated by a single trailing newline.

Shapes:

```json
{"kind":"listing","entries":[{"path":"…","kind":"task","summary":"…"}, …]}
```

```json
{"kind":"task","path":"…","summary":"…","body":"…"}
```

```json
{"kind":"directory","path":"…","summary":"…","body":"…","entries":[{"path":"…","kind":"task","summary":"…"}, …]}
```

```json
{"kind":"error","error":"collision","message":"…","paths":["…","…"]}
```

```json
{"kind":"error","error":"not_found","message":"…","name":"…","searched":["…","…"]}
```

Field rules (full detail in `contracts/ai-output.md`):

- `kind` is always present at the top level and always one of `listing` | `task` | `directory` | `error`.
- `summary` is always present on `task` / `directory` / child entry records; `null` when no summary annotation was found (NOT omitted), per FR-009.
- `body` is omitted from a record when there is no body (per FR-003a's "omitting the body section when no body is present").
- Inside a listing's `entries[]` or a directory's `entries[]`, each child has `path`, `kind` ("task" or "directory"), and `summary` (string or null). Children never carry `body` or recursive `entries` (FR-010: no recursion).
- On a collision or not-found error, `kind` is `error`, exit status is non-zero, and **no partial listing is emitted**. The error envelope is the entire output.

**Rationale**:

- Single JSON document per invocation matches how AI agents typically integrate stdout: one `JSON.parse` call. The `kind` discriminator means the parser routes once and reads shape-specific fields.
- For the spec's scale (tens to low-hundreds of entries), streaming has no real benefit. NDJSON's main advantage is unbounded streams; here the upper bound is small, payload is small, and the simpler envelope wins.
- Errors land as a structured `error` envelope, so the AI client never has to fall back to parsing stderr text to recognize a collision. FR-013's "AI form emits a structured error record" is satisfied directly.
- Stable ordering of children comes from FR-007 (directories first, then tasks, basename lexicographic) — already pinned in the spec, just enforced in the encoder.

**Alternatives considered**:

- **NDJSON (newline-delimited JSON)**: line-streamable, grep-friendly. Rejected because at this scale the streaming win is illusory, and the no-arg vs file-target vs directory-target shapes don't unify naturally (a directory record has nested children — those would have to be either inlined as a JSON array on a single line or flattened across multiple lines, breaking the "one entity per line" mental model either way).
- **TSV / pipe-delimited**: not extensible; awkward for the `body` field which can contain newlines.
- **MessagePack / protobuf**: too heavy for the use case; AI consumers expect text by convention.

---

## R4: Retirement of the legacy `list` and `ai list` subcommands

**Decision**: Delete `taskgate/cmd/list.go` and `taskgate/cmd/list_test.go` outright. Delete the `newAIListCmd` / `runAIList` functions from `taskgate/cmd/ai.go` and the corresponding tests from `taskgate/cmd/ai_test.go`. Drop the `root.AddCommand(newListCmd())` line from `taskgate/cmd/root.go` and the `aiCmd.AddCommand(newAIListCmd())` line from the `ai` group constructor. No alias, no deprecation shim, no migration notice in the binary (the change is documented in the spec, the PR description, and the release notes).

**Rationale**:

- Aligned with the user's explicit "no backwards compatibility" decision recorded in the spec's Naming note and Assumptions.
- Keeps the codebase free of dead-or-aliased commands that would otherwise hang around as ongoing maintenance surface.
- The merged-view name resolution in `show` already covers the discovery use case that `list` served, plus a strictly larger surface (summaries, bodies, directory navigation). No functional regression.

**Alternatives considered**:

- **Alias `list` → `show` for one release**: explicitly rejected by the user.
- **Print a deprecation message and call through to `show`**: rejected; the surface is different enough (sort order, audience-bucket visibility, exit codes on collision) that a transparent alias would surprise callers without actually helping them.

**Out-of-binary documentation**:

- The PR description (and any CHANGELOG) call out the removal explicitly so a downstream caller sees it before upgrading.
- The `taskgate --help` output, after the change, naturally lists `show` (and under `ai`, lists `show`) and not `list`, which is itself the strongest signal to anyone exploring the CLI.

---

## Summary of resolved unknowns

| Spec deferral | Phase 0 decision |
|---|---|
| Comment annotation notation | YAML front-matter inside a `---` envelope at the top of the file (after an optional shebang). Line-comment prefix is one of `#`, `//`, `--`, `;` (auto-detected per file from the opening delimiter); bare YAML accepted in `_index`. Keys: `summary`, `body`. Strict exact-match recognizer; YAML indentation discipline rules out the early-close failure mode in practice. |
| Directory description filename | `_index` (no extension), placed inside the directory it describes. Same YAML front-matter format; the `# ` prefix is optional inside `_index`. |
| AI wire format | Envelope JSON with `kind` discriminator (`listing` / `task` / `directory` / `error`). |
| Legacy `list` / `ai list` retirement strategy | Delete outright; no alias, no shim. |
| YAML parser dependency | `gopkg.in/yaml.v3` added to go.mod (see plan.md). |

No NEEDS CLARIFICATION markers remain. Ready for Phase 1.
