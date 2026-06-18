package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ynny-github/taskgate/taskgate/internal/show"
)

// chdirTo cd's into dir for the duration of the test and restores the
// original wd in cleanup.
func chdirTo(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
}

// writeFile creates parent dirs as needed and writes content with mode 0644.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// writeAnnotatedTask writes an executable shell task with summary/body.
func writeAnnotatedTask(t *testing.T, path, summary, body string) {
	t.Helper()
	parts := []string{"#!/bin/sh", "# ---"}
	if summary != "" {
		parts = append(parts, "# summary: "+summary)
	}
	if body != "" {
		parts = append(parts, "# body: |")
		for _, line := range strings.Split(body, "\n") {
			parts = append(parts, "#   "+line)
		}
	}
	parts = append(parts, "# ---", "echo hi", "")
	writeFile(t, path, strings.Join(parts, "\n"))
}

// wantExit asserts that err unwraps to *show.ExitError with the given code.
func wantExit(t *testing.T, err error, code int) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected *show.ExitError code=%d, got nil error", code)
	}
	var exitErr *show.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected *show.ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != code {
		t.Errorf("exit code = %d, want %d", exitErr.Code, code)
	}
}

// stageFixture writes the canonical multi-task fixture used by US1+ tests.
func stageFixture(t *testing.T, tmp string) {
	t.Helper()
	writeAnnotatedTask(t, filepath.Join(tmp, ".taskgate/human/build"), "Build the project.", "Reads VERSION.")
	writeAnnotatedTask(t, filepath.Join(tmp, ".taskgate/shared/lint"), "Lint the codebase.", "")
	writeFile(t, filepath.Join(tmp, ".taskgate/shared/test"), "#!/bin/sh\necho hi\n")
	writeAnnotatedTask(t, filepath.Join(tmp, ".taskgate/ai/analyze"), "Analyze.", "")
}

func TestShow_RootView_Human(t *testing.T) {
	tmp := t.TempDir()
	stageFixture(t, tmp)
	chdirTo(t, tmp)

	var out, errBuf bytes.Buffer
	root := newRootCmd()
	root.SetArgs([]string{"show"})
	root.SetOut(&out)
	root.SetErr(&errBuf)
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, ".taskgate/human/build") {
		t.Errorf("missing build row: %q", got)
	}
	if !strings.Contains(got, "Build the project.") {
		t.Errorf("missing build summary: %q", got)
	}
	if !strings.Contains(got, ".taskgate/shared/lint") {
		t.Errorf("missing lint row: %q", got)
	}
	if !strings.Contains(got, ".taskgate/shared/test") {
		t.Errorf("missing bare test row: %q", got)
	}
	// audience buckets MUST NOT appear as their own rows
	for _, banned := range []string{
		".taskgate/human\n", ".taskgate/shared\n", ".taskgate/ai\n",
		".taskgate/ai/analyze",
	} {
		if strings.Contains(got, banned) {
			t.Errorf("unexpected row %q in output: %q", banned, got)
		}
	}
	if errBuf.Len() != 0 {
		t.Errorf("stderr should be empty on success, got %q", errBuf.String())
	}
}

func TestShow_RootView_NoAnnotation_StillListed(t *testing.T) {
	tmp := t.TempDir()
	writeFile(t, filepath.Join(tmp, ".taskgate/shared/bare"), "#!/bin/sh\necho hi\n")
	chdirTo(t, tmp)

	var out bytes.Buffer
	root := newRootCmd()
	root.SetArgs([]string{"show"})
	root.SetOut(&out)
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), ".taskgate/shared/bare") {
		t.Errorf("bare task should be listed, got %q", out.String())
	}
}

func TestShow_RootView_Collision_Exit4(t *testing.T) {
	tmp := t.TempDir()
	writeAnnotatedTask(t, filepath.Join(tmp, ".taskgate/human/build"), "h.", "")
	writeAnnotatedTask(t, filepath.Join(tmp, ".taskgate/shared/build"), "s.", "")
	chdirTo(t, tmp)

	var out, errBuf bytes.Buffer
	root := newRootCmd()
	root.SetArgs([]string{"show"})
	root.SetOut(&out)
	root.SetErr(&errBuf)
	err := root.Execute()
	wantExit(t, err, show.ExitCollision)

	if out.Len() != 0 {
		t.Errorf("stdout must be empty on collision, got %q", out.String())
	}
	if !strings.Contains(errBuf.String(), ".taskgate/human/build") ||
		!strings.Contains(errBuf.String(), ".taskgate/shared/build") {
		t.Errorf("stderr should list both conflicting paths, got %q", errBuf.String())
	}
}

func TestShow_FileTarget_Human(t *testing.T) {
	tmp := t.TempDir()
	stageFixture(t, tmp)
	chdirTo(t, tmp)

	var out bytes.Buffer
	root := newRootCmd()
	root.SetArgs([]string{"show", "build"})
	root.SetOut(&out)
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, ".taskgate/human/build") {
		t.Errorf("missing path: %q", got)
	}
	if !strings.Contains(got, "Build the project.") {
		t.Errorf("missing summary: %q", got)
	}
	if !strings.Contains(got, "Reads VERSION.") {
		t.Errorf("missing body: %q", got)
	}
}

func TestShow_InvalidInput_FilesystemPath_Exit2(t *testing.T) {
	tmp := t.TempDir()
	stageFixture(t, tmp)
	chdirTo(t, tmp)

	for _, arg := range []string{".taskgate/human/build", "/abs/path", "./build"} {
		var out, errBuf bytes.Buffer
		root := newRootCmd()
		root.SetArgs([]string{"show", arg})
		root.SetOut(&out)
		root.SetErr(&errBuf)
		err := root.Execute()
		wantExit(t, err, show.ExitInvalidInput)
		if !strings.Contains(errBuf.String(), "run-style") {
			t.Errorf("arg=%q: stderr should mention run-style, got %q", arg, errBuf.String())
		}
	}
}

func TestShow_InvalidInput_Empty_Exit2(t *testing.T) {
	tmp := t.TempDir()
	stageFixture(t, tmp)
	chdirTo(t, tmp)

	var out, errBuf bytes.Buffer
	root := newRootCmd()
	root.SetArgs([]string{"show", ""})
	root.SetOut(&out)
	root.SetErr(&errBuf)
	err := root.Execute()
	wantExit(t, err, show.ExitInvalidInput)
}

func TestShow_NotFound_Exit3(t *testing.T) {
	tmp := t.TempDir()
	stageFixture(t, tmp)
	chdirTo(t, tmp)

	var out, errBuf bytes.Buffer
	root := newRootCmd()
	root.SetArgs([]string{"show", "no-such-task"})
	root.SetOut(&out)
	root.SetErr(&errBuf)
	err := root.Execute()
	wantExit(t, err, show.ExitNotFound)
	if !strings.Contains(errBuf.String(), "not found") {
		t.Errorf("stderr should mention not found, got %q", errBuf.String())
	}
}

// stageDirectoryFixture stages a `deploy/` directory under human with an
// _index, two task children, and one nested empty sub-directory.
func stageDirectoryFixture(t *testing.T, tmp string) {
	t.Helper()
	writeFile(t, filepath.Join(tmp, ".taskgate/human/deploy/_index"),
		"# ---\n# summary: Promote a build.\n# body: |\n#   Idempotent.\n# ---\n")
	writeAnnotatedTask(t, filepath.Join(tmp, ".taskgate/human/deploy/prod"), "Prod.", "")
	writeAnnotatedTask(t, filepath.Join(tmp, ".taskgate/human/deploy/canary"), "Canary.", "")
}

func TestShow_DirectoryTarget_Human(t *testing.T) {
	tmp := t.TempDir()
	stageDirectoryFixture(t, tmp)
	chdirTo(t, tmp)

	var out bytes.Buffer
	root := newRootCmd()
	root.SetArgs([]string{"show", "deploy"})
	root.SetOut(&out)
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := out.String()
	for _, sub := range []string{
		".taskgate/human/deploy",
		"Promote a build.", "Idempotent.",
		".taskgate/human/deploy/canary", "Canary.",
		".taskgate/human/deploy/prod", "Prod.",
	} {
		if !strings.Contains(got, sub) {
			t.Errorf("missing %q: %q", sub, got)
		}
	}
	if strings.Contains(got, "_index") {
		t.Errorf("_index should not appear: %q", got)
	}
}

func TestShow_DirectoryTarget_NoIndex_Human(t *testing.T) {
	tmp := t.TempDir()
	stageDirectoryFixture(t, tmp)
	// Remove the _index.
	if err := os.Remove(filepath.Join(tmp, ".taskgate/human/deploy/_index")); err != nil {
		t.Fatal(err)
	}
	chdirTo(t, tmp)

	var out bytes.Buffer
	root := newRootCmd()
	root.SetArgs([]string{"show", "deploy"})
	root.SetOut(&out)
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := out.String()
	if strings.Contains(got, "Promote a build.") || strings.Contains(got, "Idempotent.") {
		t.Errorf("summary/body should be omitted with no _index: %q", got)
	}
	if !strings.Contains(got, ".taskgate/human/deploy/prod") {
		t.Errorf("children still expected: %q", got)
	}
}

func TestShow_NonShellPrefix(t *testing.T) {
	tmp := t.TempDir()
	writeFile(t, filepath.Join(tmp, ".taskgate/human/devserver"),
		"#!/usr/bin/env node\n// ---\n// summary: Run the dev server.\n// body: |\n//   Restarts on file changes.\n// ---\nrequire('./server').start();\n")
	writeFile(t, filepath.Join(tmp, ".taskgate/shared/format"),
		"#!/usr/bin/env lua\n-- ---\n-- summary: Format the project.\n-- body: |\n--   Skips vendor/.\n-- ---\nrequire('formatter').run()\n")
	chdirTo(t, tmp)

	// Root listing surfaces both rows with their summaries.
	var out bytes.Buffer
	root := newRootCmd()
	root.SetArgs([]string{"show"})
	root.SetOut(&out)
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "Run the dev server.") || !strings.Contains(got, "Format the project.") {
		t.Errorf("missing summaries in root listing: %q", got)
	}

	for _, c := range []struct {
		name, wantSummary, wantBody string
	}{
		{"devserver", "Run the dev server.", "Restarts on file changes."},
		{"format", "Format the project.", "Skips vendor/."},
	} {
		var detail bytes.Buffer
		r := newRootCmd()
		r.SetArgs([]string{"show", c.name})
		r.SetOut(&detail)
		if err := r.Execute(); err != nil {
			t.Fatalf("show %s: %v", c.name, err)
		}
		gotDetail := detail.String()
		if !strings.Contains(gotDetail, c.wantSummary) {
			t.Errorf("show %s missing summary: %q", c.name, gotDetail)
		}
		if !strings.Contains(gotDetail, c.wantBody) {
			t.Errorf("show %s missing body: %q", c.name, gotDetail)
		}
	}
}

func TestShow_DescriptionFileIsolation(t *testing.T) {
	tmp := t.TempDir()
	writeAnnotatedTask(t, filepath.Join(tmp, ".taskgate/human/a/task1"), "A1.", "")
	writeAnnotatedTask(t, filepath.Join(tmp, ".taskgate/human/b/task1"), "B1.", "")
	chdirTo(t, tmp)

	// Snapshot show outputs before adding _index anywhere.
	snapshot := func(name string) string {
		t.Helper()
		var buf bytes.Buffer
		r := newRootCmd()
		r.SetArgs([]string{"show", name})
		r.SetOut(&buf)
		if err := r.Execute(); err != nil {
			t.Fatalf("show %s: %v", name, err)
		}
		return buf.String()
	}
	beforeB := snapshot("b")
	beforeA := snapshot("a")

	// Add _index only to a/.
	writeFile(t, filepath.Join(tmp, ".taskgate/human/a/_index"),
		"# ---\n# summary: A directory.\n# ---\n")

	afterA := snapshot("a")
	afterB := snapshot("b")

	if afterA == beforeA {
		t.Error("show a should change after _index added")
	}
	if !strings.Contains(afterA, "A directory.") {
		t.Errorf("show a should include new summary: %q", afterA)
	}
	if afterB != beforeB {
		t.Errorf("show b should be byte-for-byte unchanged. before=%q after=%q", beforeB, afterB)
	}
}

func TestShow_WorkspaceMissing_Exit5(t *testing.T) {
	tmp := t.TempDir() // no .taskgate/ here
	chdirTo(t, tmp)

	var out, errBuf bytes.Buffer
	root := newRootCmd()
	root.SetArgs([]string{"show"})
	root.SetOut(&out)
	root.SetErr(&errBuf)
	err := root.Execute()
	wantExit(t, err, show.ExitWorkspaceMissing)
	if !strings.Contains(errBuf.String(), ".taskgate/") {
		t.Errorf("stderr should mention .taskgate/, got %q", errBuf.String())
	}
}

func TestShow_LegacyListGone(t *testing.T) {
	tmp := t.TempDir()
	chdirTo(t, tmp)

	for _, args := range [][]string{{"list"}, {"ai", "list"}} {
		root := newRootCmd()
		root.SetArgs(args)
		var errBuf bytes.Buffer
		root.SetErr(&errBuf)
		if err := root.Execute(); err == nil {
			t.Errorf("%v should error (unknown command), got nil", args)
		}
	}
}

func TestShow_RootView_AudienceAI_Skeleton(t *testing.T) {
	tmp := t.TempDir()
	writeAnnotatedTask(t, filepath.Join(tmp, ".taskgate/ai/analyze"), "Analyze.", "")
	chdirTo(t, tmp)

	var out bytes.Buffer
	root := newRootCmd()
	root.SetArgs([]string{"ai", "show"})
	root.SetOut(&out)
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Len() == 0 {
		t.Errorf("ai show should produce non-empty output")
	}
}
