// taskgate/cmd/ai_test.go
package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeExec creates an executable file at path with the given content,
// creating parent directories as needed.
func writeExec(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestResolveAITask_NotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := resolveAITask(dir, "build")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	want := `task "build" not found in snapshot dir (` + dir + `)`
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestResolveAITask_NotExecutable(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "build"), []byte("#!/bin/sh\necho hi"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := resolveAITask(dir, "build")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != `task "build" is not executable` {
		t.Errorf("got %q, want task \"build\" is not executable", err.Error())
	}
}

func TestResolveAITask_Found(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "build")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\necho hi"), 0755); err != nil {
		t.Fatal(err)
	}
	got, err := resolveAITask(dir, "build")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != scriptPath {
		t.Errorf("got %q, want %q", got, scriptPath)
	}
}

func TestAIRunCmd_ExecutesFromSnapshot(t *testing.T) {
	snapDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(snapDir, "hello"), []byte("#!/bin/sh\necho ai-hello"), 0755); err != nil {
		t.Fatal(err)
	}

	tmp := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	snapshotDirOverride = func(string) (string, error) { return snapDir, nil }
	t.Cleanup(func() { snapshotDirOverride = nil })

	var buf bytes.Buffer
	root := newRootCmd()
	root.SetArgs([]string{"ai", "run", "hello"})
	root.SetOut(&buf)
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "ai-hello" {
		t.Errorf("output = %q, want %q", got, "ai-hello")
	}
}

func TestAIRunCmd_ErrorWhenSnapshotStale(t *testing.T) {
	snapDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(snapDir, "build"), []byte("#!/bin/sh\necho old"), 0755); err != nil {
		t.Fatal(err)
	}

	tmp := t.TempDir()
	aiDir := filepath.Join(tmp, ".taskgate", "ai")
	if err := os.MkdirAll(aiDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(aiDir, "build"), []byte("#!/bin/sh\necho new"), 0755); err != nil {
		t.Fatal(err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	snapshotDirOverride = func(string) (string, error) { return snapDir, nil }
	t.Cleanup(func() { snapshotDirOverride = nil })

	root := newRootCmd()
	root.SetArgs([]string{"ai", "run", "build"})
	err = root.Execute()
	if err == nil {
		t.Fatal("expected error for stale snapshot, got nil")
	}
	if !strings.Contains(err.Error(), "taskgate snapshot install") {
		t.Errorf("expected error to mention 'taskgate snapshot install', got: %q", err.Error())
	}
}

func TestAIRunCmd_RunsWhenSnapshotFresh(t *testing.T) {
	content := []byte("#!/bin/sh\necho fresh")
	snapDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(snapDir, "build"), content, 0755); err != nil {
		t.Fatal(err)
	}

	tmp := t.TempDir()
	aiDir := filepath.Join(tmp, ".taskgate", "ai")
	if err := os.MkdirAll(aiDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(aiDir, "build"), content, 0755); err != nil {
		t.Fatal(err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	snapshotDirOverride = func(string) (string, error) { return snapDir, nil }
	t.Cleanup(func() { snapshotDirOverride = nil })

	root := newRootCmd()
	root.SetArgs([]string{"ai", "run", "build"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error for fresh snapshot: %v", err)
	}
}

func TestAIRunCmd_RunsWhenSourceMissing(t *testing.T) {
	snapDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(snapDir, "build"), []byte("#!/bin/sh\necho hi"), 0755); err != nil {
		t.Fatal(err)
	}

	tmp := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	snapshotDirOverride = func(string) (string, error) { return snapDir, nil }
	t.Cleanup(func() { snapshotDirOverride = nil })

	root := newRootCmd()
	root.SetArgs([]string{"ai", "run", "build"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error when no source file: %v", err)
	}
}

func TestAIRunCmd_ErrorWhenSnapshotMissing(t *testing.T) {
	snapDir := t.TempDir()

	tmp := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	snapshotDirOverride = func(string) (string, error) { return snapDir, nil }
	t.Cleanup(func() { snapshotDirOverride = nil })

	root := newRootCmd()
	root.SetArgs([]string{"ai", "run", "missing"})
	err = root.Execute()
	if err == nil {
		t.Fatal("expected error for missing snapshot task, got nil")
	}
}

func TestAIRunCmd_PassesFlagStyleArgs(t *testing.T) {
	content := []byte("#!/bin/sh\necho \"$@\"")
	snapDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(snapDir, "print-args"), content, 0755); err != nil {
		t.Fatal(err)
	}

	tmp := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	snapshotDirOverride = func(string) (string, error) { return snapDir, nil }
	t.Cleanup(func() { snapshotDirOverride = nil })

	var buf bytes.Buffer
	root := newRootCmd()
	root.SetArgs([]string{"ai", "run", "print-args", "--env", "prod"})
	root.SetOut(&buf)
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "--env prod" {
		t.Errorf("got %q, want %q", got, "--env prod")
	}
}

func TestSnapshotDirFn_XDGStateHome(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".taskgate"), 0755); err != nil {
		t.Fatal(err)
	}
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)

	got, err := snapshotDirFn(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(state, "taskgate", "snapshots", snapshotDirName(tmp))
	if got != want {
		t.Errorf("snapshotDirFn = %q, want %q", got, want)
	}
}

func TestSnapshotDirFn_DefaultsToLocalState(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".taskgate"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_STATE_HOME", "")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	got, err := snapshotDirFn(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(home, ".local", "state", "taskgate", "snapshots", snapshotDirName(tmp))
	if got != want {
		t.Errorf("snapshotDirFn = %q, want %q", got, want)
	}
}

func TestSnapshotDirFn_NoRoot(t *testing.T) {
	tmp := t.TempDir() // no .taskgate anywhere
	_, err := snapshotDirFn(tmp)
	if err == nil {
		t.Fatal("expected error when no .taskgate marker, got nil")
	}
	want := "cannot determine project root: .taskgate directory not found"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

// makeTask creates an executable task script under .taskgate/<subdir>/<name>.
// Lives here because list_test.go (which previously hosted it) is gone.
func makeTask(t *testing.T, tmp, subdir, name string) {
	t.Helper()
	dir := filepath.Join(tmp, ".taskgate", subdir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\necho hi"), 0755); err != nil {
		t.Fatal(err)
	}
}

func TestAIRun_ExecutesBeforeDependency(t *testing.T) {
	tmp := t.TempDir()
	snap := filepath.Join(tmp, "snap")
	order := filepath.Join(tmp, "order.txt")
	if err := os.MkdirAll(snap, 0o755); err != nil {
		t.Fatal(err)
	}
	// snapshot copies (what ai run executes)
	writeExec(t, filepath.Join(snap, "build"), "#!/bin/sh\necho build >> "+order+"\n")
	writeExec(t, filepath.Join(snap, "deploy"),
		"#!/bin/sh\n# ---\n# before:\n#   - build\n# ---\necho deploy >> "+order+"\n")
	// matching sources under .taskgate so freshness passes
	writeExec(t, filepath.Join(tmp, ".taskgate", "ai", "build"), "#!/bin/sh\necho build >> "+order+"\n")
	writeExec(t, filepath.Join(tmp, ".taskgate", "ai", "deploy"),
		"#!/bin/sh\n# ---\n# before:\n#   - build\n# ---\necho deploy >> "+order+"\n")

	snapshotDirOverride = func(string) (string, error) { return snap, nil }
	defer func() { snapshotDirOverride = nil }()

	var stdout, stderr bytes.Buffer
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmp)
	code := Run([]string{"ai", "run", "deploy"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}
	got, _ := os.ReadFile(order)
	if strings.TrimSpace(string(got)) != "build\ndeploy" {
		t.Fatalf("order = %q, want build\\ndeploy", got)
	}
}

func TestAIRun_StaleDependencyErrors(t *testing.T) {
	tmp := t.TempDir()
	snap := filepath.Join(tmp, "snap")
	if err := os.MkdirAll(snap, 0o755); err != nil {
		t.Fatal(err)
	}
	writeExec(t, filepath.Join(snap, "build"), "#!/bin/sh\necho OLD\n")
	writeExec(t, filepath.Join(snap, "deploy"),
		"#!/bin/sh\n# ---\n# before:\n#   - build\n# ---\necho deploy\n")
	writeExec(t, filepath.Join(tmp, ".taskgate", "ai", "build"), "#!/bin/sh\necho NEW\n") // differs → stale
	writeExec(t, filepath.Join(tmp, ".taskgate", "ai", "deploy"),
		"#!/bin/sh\n# ---\n# before:\n#   - build\n# ---\necho deploy\n")

	snapshotDirOverride = func(string) (string, error) { return snap, nil }
	defer func() { snapshotDirOverride = nil }()

	var stdout, stderr bytes.Buffer
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmp)
	code := Run([]string{"ai", "run", "deploy"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit for stale dependency")
	}
	if !strings.Contains(stderr.String(), "out of date") {
		t.Fatalf("stderr %q should report the stale snapshot", stderr.String())
	}
}
