package annotation

import (
	"strings"
	"testing"
)

func parseString(t *testing.T, src string) AnnotationBlock {
	t.Helper()
	block, err := Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	return block
}

func TestParse_ShellPrefix(t *testing.T) {
	got := parseString(t, `#!/bin/sh
# ---
# summary: Build the project.
# body: |
#   Reads VERSION from env.
#   Exits non-zero on failure.
# ---

set -e
echo hi
`)
	if got.Summary != "Build the project." {
		t.Errorf("summary = %q", got.Summary)
	}
	wantBody := "Reads VERSION from env.\nExits non-zero on failure."
	if got.Body != wantBody {
		t.Errorf("body = %q, want %q", got.Body, wantBody)
	}
}

func TestParse_DoubleSlashPrefix(t *testing.T) {
	got := parseString(t, `#!/usr/bin/env node
// ---
// summary: Run the dev server.
// body: |
//   Restarts on file changes.
// ---

require('./server').start();
`)
	if got.Summary != "Run the dev server." {
		t.Errorf("summary = %q", got.Summary)
	}
	if got.Body != "Restarts on file changes." {
		t.Errorf("body = %q", got.Body)
	}
}

func TestParse_DoubleDashPrefix(t *testing.T) {
	got := parseString(t, `#!/usr/bin/env lua
-- ---
-- summary: Format the project.
-- body: |
--   Skips vendor/.
-- ---

require('formatter').run()
`)
	if got.Summary != "Format the project." {
		t.Errorf("summary = %q", got.Summary)
	}
	if got.Body != "Skips vendor/." {
		t.Errorf("body = %q", got.Body)
	}
}

func TestParse_SemicolonPrefix(t *testing.T) {
	got := parseString(t, `; ---
; summary: Initialize the Lisp environment.
; ---

(load "init.lisp")
`)
	if got.Summary != "Initialize the Lisp environment." {
		t.Errorf("summary = %q", got.Summary)
	}
	if got.Body != "" {
		t.Errorf("body = %q, want empty", got.Body)
	}
}

func TestParse_BareYAML(t *testing.T) {
	got := parseString(t, `---
summary: Promote a build to an environment.
body: |
  Each child task corresponds to a deploy target.
---
`)
	if got.Summary != "Promote a build to an environment." {
		t.Errorf("summary = %q", got.Summary)
	}
	if got.Body != "Each child task corresponds to a deploy target." {
		t.Errorf("body = %q", got.Body)
	}
}

func TestParse_MissingOpener(t *testing.T) {
	got := parseString(t, `#!/bin/sh
# just a comment, not an envelope
echo hi
`)
	if got != (AnnotationBlock{}) {
		t.Errorf("got %+v, want zero", got)
	}
}

func TestParse_MissingCloser(t *testing.T) {
	got := parseString(t, `#!/bin/sh
# ---
# summary: never closes
echo hi
`)
	if got != (AnnotationBlock{}) {
		t.Errorf("got %+v, want zero", got)
	}
}

func TestParse_MismatchedPrefix(t *testing.T) {
	got := parseString(t, `#!/bin/sh
# ---
# summary: opened with hash
// ---
`)
	if got != (AnnotationBlock{}) {
		t.Errorf("got %+v, want zero (closer was wrong prefix)", got)
	}
}

func TestParse_MalformedYAML(t *testing.T) {
	got := parseString(t, `#!/bin/sh
# ---
# summary: : :: not parseable
#   - bad
#  weird indent
# ---
`)
	if got != (AnnotationBlock{}) {
		t.Errorf("got %+v, want zero on malformed YAML", got)
	}
}

func TestParse_MultilineBody(t *testing.T) {
	got := parseString(t, `# ---
# summary: top
# body: |
#   line one
#   line two
#
#   line four (after blank)
# ---
`)
	want := "line one\nline two\n\nline four (after blank)"
	if got.Body != want {
		t.Errorf("body = %q, want %q", got.Body, want)
	}
}

func TestParse_SingleLineSummary(t *testing.T) {
	got := parseString(t, `# ---
# summary: just a summary
# ---
`)
	if got.Summary != "just a summary" {
		t.Errorf("summary = %q", got.Summary)
	}
	if got.Body != "" {
		t.Errorf("body = %q, want empty", got.Body)
	}
}

func TestParse_EmptySummary(t *testing.T) {
	got := parseString(t, `# ---
# summary:
# ---
`)
	if got.Summary != "" {
		t.Errorf("summary = %q, want empty", got.Summary)
	}
}

func TestParse_UnknownKeysIgnored(t *testing.T) {
	got := parseString(t, `# ---
# summary: hi
# tags:
#   - a
#   - b
# author: someone
# ---
`)
	if got.Summary != "hi" {
		t.Errorf("summary = %q", got.Summary)
	}
}

func TestParse_ShebangSkipped(t *testing.T) {
	got := parseString(t, `#!/usr/bin/env python3
# ---
# summary: py task
# ---
`)
	if got.Summary != "py task" {
		t.Errorf("summary = %q", got.Summary)
	}
}

func TestParse_NoEnvelope(t *testing.T) {
	got := parseString(t, "#!/bin/sh\necho hi\n")
	if got != (AnnotationBlock{}) {
		t.Errorf("got %+v, want zero", got)
	}
}

func TestParse_DelimiterNotStrictMatch(t *testing.T) {
	// "# ----" (four dashes) and "# --- foo" should not open the envelope.
	got := parseString(t, `# ----
# summary: nope
# ----
`)
	if got != (AnnotationBlock{}) {
		t.Errorf("got %+v, want zero (four-dash not a delimiter)", got)
	}

	got = parseString(t, `# --- foo
# summary: nope
# --- foo
`)
	if got != (AnnotationBlock{}) {
		t.Errorf("got %+v, want zero (trailing token not a delimiter)", got)
	}
}

func TestParse_PrecedingCommentLinesIgnored(t *testing.T) {
	// shellcheck-style pragmas and copyright headers before the envelope
	// must not be mistaken for the opener.
	got := parseString(t, `#!/bin/sh
# shellcheck disable=SC2086
# copyright (c) 2026
# ---
# summary: build
# ---
`)
	if got.Summary != "build" {
		t.Errorf("summary = %q", got.Summary)
	}
}
