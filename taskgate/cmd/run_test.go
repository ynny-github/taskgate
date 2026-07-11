// taskgate/cmd/run_test.go
package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeHumanScript(t *testing.T, tmp, name, content string) string {
	t.Helper()
	dir := filepath.Join(tmp, ".taskgate", "human")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestResolveHumanTask_TaskMissing(t *testing.T) {
	tmp := t.TempDir()
	_, err := resolveHumanTask(tmp, "build")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	want := `task "build" not found in .taskgate/human/ or .taskgate/shared/`
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestResolveHumanTask_NotExecutable(t *testing.T) {
	tmp := t.TempDir()
	p := makeHumanScript(t, tmp, "build", "#!/bin/sh\necho hi")
	if err := os.Chmod(p, 0644); err != nil {
		t.Fatal(err)
	}
	_, err := resolveHumanTask(tmp, "build")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != `task "build" is not executable` {
		t.Errorf("got %q, want task \"build\" is not executable", err.Error())
	}
}

func TestResolveHumanTask_HumanDir(t *testing.T) {
	tmp := t.TempDir()
	scriptPath := makeHumanScript(t, tmp, "build", "#!/bin/sh\necho hi")
	got, err := resolveHumanTask(tmp, "build")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != scriptPath {
		t.Errorf("got %q, want %q", got, scriptPath)
	}
}

func TestResolveHumanTask_SharedDir(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, ".taskgate", "shared")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(dir, "build")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\necho hi"), 0755); err != nil {
		t.Fatal(err)
	}
	got, err := resolveHumanTask(tmp, "build")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != scriptPath {
		t.Errorf("got %q, want %q", got, scriptPath)
	}
}

func TestRunCmd_ExecutesScript(t *testing.T) {
	tmp := t.TempDir()
	makeHumanScript(t, tmp, "hello", "#!/bin/sh\necho hello-from-taskgate")

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	root := newRootCmd()
	root.SetArgs([]string{"run", "hello"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunCmd_PropagatesExitCode(t *testing.T) {
	tmp := t.TempDir()
	makeHumanScript(t, tmp, "fail", "#!/bin/sh\nexit 42")

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	root := newRootCmd()
	root.SetArgs([]string{"run", "fail"})
	err = root.Execute()
	if err == nil {
		t.Fatal("expected non-nil error for exit code 42")
	}
	var exitErr *exitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected *exitError, got %T: %v", err, err)
	}
	if exitErr.code != 42 {
		t.Errorf("exit code = %d, want 42", exitErr.code)
	}
}

func TestRunCmd_SetsProjectRoot(t *testing.T) {
	tmp := t.TempDir()
	realTmp, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		t.Fatal(err)
	}
	makeHumanScript(t, realTmp, "print-root", "#!/bin/sh\necho \"$TASKGATE_PROJECT_ROOT\"")

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(realTmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	var buf bytes.Buffer
	root := newRootCmd()
	root.SetArgs([]string{"run", "print-root"})
	root.SetOut(&buf)
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if got != realTmp {
		t.Errorf("TASKGATE_PROJECT_ROOT = %q, want %q", got, realTmp)
	}
}

func TestRunCmd_ResolvesTaskFromSubdir(t *testing.T) {
	tmp := t.TempDir()
	realTmp, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		t.Fatal(err)
	}
	// Create .taskgate/human/ at the root.
	makeHumanScript(t, realTmp, "print-root", "#!/bin/sh\necho \"$TASKGATE_PROJECT_ROOT\"")

	// Create a subdirectory and change into it.
	subdir := filepath.Join(realTmp, "sub")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatal(err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(subdir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	// Run from the subdirectory; the project root should be found in the parent.
	var buf bytes.Buffer
	root := newRootCmd()
	root.SetArgs([]string{"run", "print-root"})
	root.SetOut(&buf)
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if got != realTmp {
		t.Errorf("TASKGATE_PROJECT_ROOT = %q, want %q (parent project root)", got, realTmp)
	}
}

func TestRunCmd_NoMarker_TaskNotFound(t *testing.T) {
	tmp := t.TempDir()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	root := newRootCmd()
	root.SetArgs([]string{"run", "ghost"})
	err = root.Execute()
	if err == nil {
		t.Fatal("expected error when no .taskgate marker exists, got nil")
	}
	want := `dependency "ghost": not found in .taskgate/human/ or .taskgate/shared/`
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestRunCmd_PassesFlagStyleArgs(t *testing.T) {
	tmp := t.TempDir()
	makeHumanScript(t, tmp, "print-args", "#!/bin/sh\necho \"$@\"")

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	var buf bytes.Buffer
	root := newRootCmd()
	root.SetArgs([]string{"run", "print-args", "--foo", "bar"})
	root.SetOut(&buf)
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "--foo bar" {
		t.Errorf("got %q, want %q", got, "--foo bar")
	}
}

func TestDetectProjectRoot_FindsMarkerInParent(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".taskgate"), 0755); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(tmp, "a", "b")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	got := detectProjectRoot(sub)
	if got != tmp {
		t.Errorf("detectProjectRoot(%q) = %q, want %q", sub, got, tmp)
	}
}

func TestDetectProjectRoot_NoMarkerReturnsEmpty(t *testing.T) {
	tmp := t.TempDir()
	sub := filepath.Join(tmp, "a", "b")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	got := detectProjectRoot(sub)
	if got != "" {
		t.Errorf("detectProjectRoot(%q) = %q, want empty", sub, got)
	}
}

func TestDetectProjectRoot_NearestAncestorWins(t *testing.T) {
	tmp := t.TempDir()
	outer := filepath.Join(tmp, "outer")
	inner := filepath.Join(outer, "inner")
	if err := os.MkdirAll(filepath.Join(outer, ".taskgate"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(inner, ".taskgate"), 0755); err != nil {
		t.Fatal(err)
	}
	start := filepath.Join(inner, "x")
	if err := os.MkdirAll(start, 0755); err != nil {
		t.Fatal(err)
	}
	got := detectProjectRoot(start)
	if got != inner {
		t.Errorf("detectProjectRoot(%q) = %q, want %q", start, got, inner)
	}
}

func TestDetectProjectRoot_IgnoresMarkerFile(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, ".taskgate"), []byte("not a dir"), 0644); err != nil {
		t.Fatal(err)
	}
	got := detectProjectRoot(tmp)
	if got != "" {
		t.Errorf("detectProjectRoot(%q) = %q, want empty (.taskgate is a file)", tmp, got)
	}
}

func TestRun_ExecutesBeforeDependency(t *testing.T) {
	tmp := t.TempDir()
	order := filepath.Join(tmp, "order.txt")
	makeHumanScript(t, tmp, "build", "#!/bin/sh\necho build >> "+order+"\n")
	makeHumanScript(t, tmp, "deploy",
		"#!/bin/sh\n# ---\n# before:\n#   - build\n# ---\necho deploy >> "+order+"\n")

	var stdout, stderr bytes.Buffer
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmp)
	code := Run([]string{"run", "deploy"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}
	got, _ := os.ReadFile(order)
	if strings.TrimSpace(string(got)) != "build\ndeploy" {
		t.Fatalf("order = %q, want build\\ndeploy", got)
	}
}

func TestRun_BeforeFailureAborts(t *testing.T) {
	tmp := t.TempDir()
	order := filepath.Join(tmp, "order.txt")
	makeHumanScript(t, tmp, "build", "#!/bin/sh\necho build >> "+order+"\nexit 7\n")
	makeHumanScript(t, tmp, "deploy",
		"#!/bin/sh\n# ---\n# before:\n#   - build\n# ---\necho deploy >> "+order+"\n")

	var stdout, stderr bytes.Buffer
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmp)
	code := Run([]string{"run", "deploy"}, &stdout, &stderr)
	if code != 7 {
		t.Fatalf("exit = %d, want 7", code)
	}
	got, _ := os.ReadFile(order)
	if strings.TrimSpace(string(got)) != "build" {
		t.Fatalf("order = %q, want build only", got)
	}
}

func TestRun_UnknownDependencyErrors(t *testing.T) {
	tmp := t.TempDir()
	makeHumanScript(t, tmp, "deploy",
		"#!/bin/sh\n# ---\n# before:\n#   - ghost\n# ---\necho deploy\n")
	var stdout, stderr bytes.Buffer
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmp)
	code := Run([]string{"run", "deploy"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit for unknown dependency")
	}
	if !strings.Contains(stderr.String(), "ghost") {
		t.Fatalf("stderr %q should mention the unknown dep", stderr.String())
	}
}
