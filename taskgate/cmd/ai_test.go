// taskgate/cmd/ai_test.go
package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

	var errBuf bytes.Buffer
	root := newRootCmd()
	root.SetArgs([]string{"ai", "run", "build"})
	root.SetErr(&errBuf)
	err = root.Execute()
	if err == nil {
		t.Fatal("expected error for stale snapshot, got nil")
	}
	if !strings.Contains(errBuf.String(), "taskgate snapshot install") {
		t.Errorf("expected stderr to mention 'taskgate snapshot install', got: %q", errBuf.String())
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
