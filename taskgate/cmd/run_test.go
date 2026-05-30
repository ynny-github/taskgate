// taskgate/cmd/run_test.go
package cmd

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
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
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected *exec.ExitError, got %T: %v", err, err)
	}
	if exitErr.ExitCode() != 42 {
		t.Errorf("exit code = %d, want 42", exitErr.ExitCode())
	}
}

func TestRunCmd_SetsProjectRoot(t *testing.T) {
	tmp := t.TempDir()
	realTmp, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "init", realTmp).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
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

func TestRunCmd_NoProjectRoot_OutsideRepo(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", tmp)
	makeHumanScript(t, tmp, "print-root", "#!/bin/sh\necho \"${TASKGATE_PROJECT_ROOT:-UNSET}\"")

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
	root.SetArgs([]string{"run", "print-root"})
	root.SetOut(&buf)
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "UNSET" {
		t.Errorf("expected TASKGATE_PROJECT_ROOT to be unset, got %q", got)
	}
}
