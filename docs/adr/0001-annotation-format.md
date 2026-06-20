# ADR-0001: Annotation format for task descriptions

**Status**: Accepted (2026-06-16)

## Context

Task files under `.taskgate/{human,ai,shared}/` need a way for authors to attach a short summary (single line) and an optional body (multi-line) so that `taskgate show` can present them. The annotation lives **inside** the task script file (not a sidecar) so each task stays self-contained.

The annotation must:

- Survive the script's own runtime — it must be valid syntax in the script's language (sh, JavaScript, Lua, etc.).
- Allow multi-line content cleanly.
- Be extensible so future fields (`tags`, `since`, `author`) can drop in without re-engineering the parser.
- Be easy for humans **and** AI assistants to author without escape acrobatics.

Multiple script languages are in scope (any shebang-invocable interpreter), so the format cannot assume a single comment style.

## Decision

**YAML front-matter inside a leading comment block, delimited by `---` lines at both ends.** The delimiters and every line inside the envelope carry a line-comment prefix appropriate to the script's language. The parser auto-detects the prefix from the opening delimiter — the first matching prefix is "the prefix" for that file.

| Prefix | Languages |
|---|---|
| `#` | sh, bash, zsh, python, ruby, perl, makefile, yaml |
| `//` | JavaScript, TypeScript, Go (as `go run` script), Java, Rust, Swift, C-family |
| `--` | Lua, SQL, Haskell, Ada |
| `;` | Lisp / Clojure / Scheme, INI-style |
| (none) | `_index` files written as bare YAML (no shebang, not executed) |

Recognized top-level YAML keys: `summary` (string), `body` (string). All other keys are silently ignored (forward-compatibility).

Block-style comments (`/* ... */`, `<!-- ... -->`) are intentionally out of scope.

Example (sh):

```sh
#!/bin/sh
# ---
# summary: Build the project for the current platform.
# body: |
#   Reads VERSION from the environment.
# ---
set -euo pipefail
go build -o bin/taskgate ./taskgate
```

## Consequences

**Positive**:

- Uses the YAML front-matter convention already pinned by Hugo, Jekyll, Pandoc, MkDocs, etc. — annotations can be parsed by any of those tools with a one-pass prefix strip.
- Bare `---` delimiter (no tool-name suffix like `---taskgate`) keeps the on-disk format portable across project renames or forks.
- The literal-block scalar `body: |` handles multi-line content without escape gymnastics, which matters for AI-generated annotations.
- Each script file remains valid in its host language — every annotation line carries the script's own line-comment prefix, so the kernel still honors the shebang and language-specific tools (shellcheck, eslint, luacheck) are unaffected.
- Forward-compatible: new keys (`tags`, `since`, `author`, `deprecated`) drop in without parser changes.

**Negative**:

- Adds `gopkg.in/yaml.v3` as a module dependency. Trade-off accepted vs. a fragile hand-rolled parser.
- A `<prefix> ---` line inside a `body: |` literal-block at the wrong indentation closes the envelope early. Mitigated by exact-match recognition (the YAML-required two-space indent under `body:` strips to `  ---`, which does not match the bare `---` delimiter) and by YAML itself rejecting a column-0 `---` inside a literal-block.

## Alternatives considered

- **Tag-based `#: ` / `#:: ` markers**: more compact, no YAML dependency. Rejected because multi-line `body` continuation is brittle when an AI writes it (must remember to prefix every continuation line, easy to forget); not extensible without inventing more markers.
- **Implicit "first comment block is the description"**: zero syntax. Rejected because every leading shellcheck pragma or copyright header is silently consumed as the description; no way to write an ordinary leading comment.
- **Tag-based `# @summary:` / `# @body:` (Doxygen-style)**: familiar but verbose; body continuation rules vary across communities.
- **Sidecar Markdown file (`<name>.md` next to the script)**: cleanest separation but doubles file count and adds script/sidecar drift risk; rejected in favor of in-file front-matter to keep each task self-contained.
