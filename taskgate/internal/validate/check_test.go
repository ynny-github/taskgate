package validate

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string, mode os.FileMode) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	return p
}

func rulesOf(findings []Finding) map[string]bool {
	out := map[string]bool{}
	for _, f := range findings {
		out[f.Rule] = true
	}
	return out
}

func TestCheckFile_CleanTaskHasNoFindings(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "build", "#!/bin/sh\n# ---\n# summary: Build.\n# ---\necho hi\n", 0o755)
	got, err := checkFile(discovered{absPath: p, displayPath: ".taskgate/shared/build", logicalName: "build"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no findings, got %+v", got)
	}
}

func TestCheckFile_NonExecutableTask(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "build", "#!/bin/sh\necho hi\n", 0o644)
	got, err := checkFile(discovered{absPath: p, displayPath: ".taskgate/shared/build", logicalName: "build"})
	if err != nil {
		t.Fatal(err)
	}
	if !rulesOf(got)[RuleExecBit] {
		t.Fatalf("expected exec-bit finding, got %+v", got)
	}
}

func TestCheckFile_MissingShebang(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "build", "echo hi\n", 0o755)
	got, err := checkFile(discovered{absPath: p, displayPath: ".taskgate/shared/build", logicalName: "build"})
	if err != nil {
		t.Fatal(err)
	}
	if !rulesOf(got)[RuleShebang] {
		t.Fatalf("expected shebang finding, got %+v", got)
	}
}

func TestCheckFile_BrokenAnnotation(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "build", "#!/bin/sh\n# ---\n# summary: unterminated\necho hi\n", 0o755)
	got, err := checkFile(discovered{absPath: p, displayPath: ".taskgate/shared/build", logicalName: "build"})
	if err != nil {
		t.Fatal(err)
	}
	if !rulesOf(got)[RuleAnnotation] {
		t.Fatalf("expected annotation finding, got %+v", got)
	}
}

func TestCheckFile_IndexSkipsExecAndShebang(t *testing.T) {
	dir := t.TempDir()
	// _index: non-executable, no shebang, but a valid annotation -> no findings.
	p := writeFile(t, dir, "_index", "---\nsummary: A directory.\n---\n", 0o644)
	got, err := checkFile(discovered{absPath: p, displayPath: ".taskgate/shared/deploy/_index", logicalName: "deploy", isIndex: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no findings for clean _index, got %+v", got)
	}
}

func TestCheckFile_IndexStillChecksAnnotation(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "_index", "---\nsummary: unterminated\n", 0o644)
	got, err := checkFile(discovered{absPath: p, displayPath: ".taskgate/shared/deploy/_index", logicalName: "deploy", isIndex: true})
	if err != nil {
		t.Fatal(err)
	}
	if !rulesOf(got)[RuleAnnotation] {
		t.Fatalf("expected annotation finding for broken _index, got %+v", got)
	}
}

func TestCheckFile_AbsentAnnotationIsNotAFinding(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "build", "#!/bin/sh\necho hi\n", 0o755)
	got, err := checkFile(discovered{absPath: p, displayPath: ".taskgate/shared/build", logicalName: "build"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("absent annotation must yield no findings, got %+v", got)
	}
}

func TestCheckFile_SetsLogicalName(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "build", "echo hi\n", 0o644)
	got, err := checkFile(discovered{absPath: p, displayPath: ".taskgate/shared/build", logicalName: "build"})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range got {
		if f.logical != "build" {
			t.Errorf("logical = %q, want build", f.logical)
		}
	}
}
