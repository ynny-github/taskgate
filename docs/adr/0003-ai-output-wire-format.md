# ADR-0003: AI output wire format

**Status**: Accepted (2026-06-16)

## Context

`taskgate ai show` produces machine-readable output for AI agents. The output shape must:

- Be uniform enough that a single small parser handles all three invocation cases (no-argument listing, file target, directory target) plus error envelopes.
- Carry structured errors (so AI clients don't have to parse stderr text to recognize a collision or not-found).
- Be parseable by mainstream AI client SDKs (text in, decoded structure out) without exotic libraries.

Scale: tens to low-hundreds of entries per invocation. Not a streaming protocol.

## Decision

**Single-document envelope JSON** with a top-level `"kind"` discriminator. One JSON document per `taskgate ai show` invocation, on stdout, terminated by a single trailing newline.

Four shapes:

```json
{"kind":"listing",   "audience":"…", "entries":[ {"path":"…","kind":"task|directory","summary":"…|null"}, … ]}
{"kind":"task",      "path":"…", "summary":"…|null", "body":"…",       "audience":"…"}
{"kind":"directory", "path":"…", "summary":"…|null", "body":"…",       "audience":"…", "entries":[ … ]}
{"kind":"error",     "error":"<code>", "message":"…", …}
```

Field rules:

- `kind` always present at the top level, one of `listing` / `task` / `directory` / `error`.
- `summary` is always present on entry-describing records; `null` (not omitted) when no annotation was extracted.
- `body` is omitted when no body annotation; never `null`.
- Children in `entries[]` carry `path` + `kind` + `summary` only — never `body`, never recursive `entries`.
- On any error, exit is non-zero AND a structured `error` envelope is emitted on **stdout** (not stderr), so AI clients can `JSON.parse` a single stream regardless of outcome. No partial listing on error.

Stability: the schema is **additive**. New fields may be added in future releases; removing or renaming a field is a breaking change. Consumers MUST ignore unknown fields and SHOULD treat unrecognized top-level `kind` values as a forward-compat soft failure.

The per-shape contract details live in `taskgate/testdata/show/*.txtar` headers (the executable contract).

## Consequences

**Positive**:

- One `JSON.parse` per invocation in the AI client. The `kind` discriminator routes once.
- Errors land as a structured envelope on stdout, so AI clients never fall back to parsing human-readable stderr.
- Adding a future `kind` (e.g. `"summary"` aggregating across directories) doesn't break parsers that route on the discriminator and fall through cleanly on unknowns.

**Negative**:

- Not line-streamable — `taskgate ai show` must fully render before emitting. Fine at this scale; would matter only if we ever supported thousands-of-entries listings.
- Forward-compatibility relies on consumer discipline (ignore unknown fields, soft-fail on unrecognized `kind`). Recorded as a `MUST`/`SHOULD` in the README of the testdata suite.

## Alternatives considered

- **NDJSON (newline-delimited JSON)**: line-streamable, grep-friendly. Rejected because at this scale the streaming win is illusory, and the directory record's nested `entries[]` doesn't unify naturally — either inline a JSON array on one line (breaks the "one entity per line" mental model) or flatten across lines (loses the structural relationship).
- **TSV / pipe-delimited**: not extensible; awkward for the `body` field which can contain newlines.
- **MessagePack / protobuf**: too heavy for the use case; AI consumers expect text by convention.
