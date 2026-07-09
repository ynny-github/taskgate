// taskgate/cmd/snapshot_test.go
package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeScript(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0755); err != nil {
		t.Fatal(err)
	}
}

func TestSnapshotInstall_CopiesAIAndShared(t *testing.T) {
	tmp := t.TempDir()
	makeScript(t, filepath.Join(tmp, ".taskgate", "ai"), "deploy", "#!/bin/sh\necho deploy")
	makeScript(t, filepath.Join(tmp, ".taskgate", "shared"), "build", "#!/bin/sh\necho build")

	snapDir := t.TempDir()
	snapshotDirOverride = func(string) (string, error) { return snapDir, nil }
	t.Cleanup(func() { snapshotDirOverride = nil })

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	root := newRootCmd()
	root.SetArgs([]string{"snapshot", "install"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, name := range []string{"deploy", "build"} {
		if _, err := os.Stat(filepath.Join(snapDir, name)); err != nil {
			t.Errorf("expected %s in snapshot dir: %v", name, err)
		}
	}
}

func TestSnapshotInstall_CopiesNestedDirectories(t *testing.T) {
	tmp := t.TempDir()
	makeScript(t, filepath.Join(tmp, ".taskgate", "ai", "group"), "deploy", "#!/bin/sh\necho deploy")
	makeScript(t, filepath.Join(tmp, ".taskgate", "shared", "lib", "nested"), "build", "#!/bin/sh\necho build")

	snapDir := t.TempDir()
	snapshotDirOverride = func(string) (string, error) { return snapDir, nil }
	t.Cleanup(func() { snapshotDirOverride = nil })

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	var out bytes.Buffer
	root := newRootCmd()
	root.SetOut(&out)
	root.SetArgs([]string{"snapshot", "install"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, rel := range []string{
		filepath.Join("group", "deploy"),
		filepath.Join("lib", "nested", "build"),
	} {
		if _, err := os.Stat(filepath.Join(snapDir, rel)); err != nil {
			t.Errorf("expected %s in snapshot dir: %v", rel, err)
		}
	}
	if !strings.Contains(out.String(), "installed 2 script(s)") {
		t.Errorf("expected count of 2, got %q", out.String())
	}
}

func TestSnapshotInstall_ErrorOnCollision(t *testing.T) {
	tmp := t.TempDir()
	makeScript(t, filepath.Join(tmp, ".taskgate", "ai"), "build", "#!/bin/sh\necho ai-build")
	makeScript(t, filepath.Join(tmp, ".taskgate", "shared"), "build", "#!/bin/sh\necho shared-build")

	snapDir := t.TempDir()
	snapshotDirOverride = func(string) (string, error) { return snapDir, nil }
	t.Cleanup(func() { snapshotDirOverride = nil })

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	root := newRootCmd()
	root.SetArgs([]string{"snapshot", "install"})
	err = root.Execute()
	if err == nil {
		t.Fatal("expected error for name collision, got nil")
	}
}

func TestSnapshotInstall_PreservesPermissions(t *testing.T) {
	tmp := t.TempDir()
	makeScript(t, filepath.Join(tmp, ".taskgate", "ai"), "deploy", "#!/bin/sh\necho deploy")

	snapDir := t.TempDir()
	snapshotDirOverride = func(string) (string, error) { return snapDir, nil }
	t.Cleanup(func() { snapshotDirOverride = nil })

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	root := newRootCmd()
	root.SetArgs([]string{"snapshot", "install"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, err := os.Stat(filepath.Join(snapDir, "deploy"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0111 == 0 {
		t.Errorf("expected copied file to be executable, got mode %v", info.Mode())
	}
}

func TestSnapshotInstall_CreatesSnapshotDir(t *testing.T) {
	tmp := t.TempDir()
	makeScript(t, filepath.Join(tmp, ".taskgate", "ai"), "deploy", "#!/bin/sh\necho deploy")

	snapDir := filepath.Join(t.TempDir(), "new-subdir")
	snapshotDirOverride = func(string) (string, error) { return snapDir, nil }
	t.Cleanup(func() { snapshotDirOverride = nil })

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	root := newRootCmd()
	root.SetArgs([]string{"snapshot", "install"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(snapDir); err != nil {
		t.Errorf("expected snapshot dir to be created: %v", err)
	}
}

func TestSnapshotDirName(t *testing.T) {
	a := snapshotDirName("/Users/yn/work/taskgate")
	b := snapshotDirName("/Users/yn/other/taskgate")

	if len(a) != 12 {
		t.Errorf("expected 12-char name, got %q (len %d)", a, len(a))
	}
	if a != snapshotDirName("/Users/yn/work/taskgate") {
		t.Error("expected stable name for the same root")
	}
	if a == b {
		t.Error("expected different names for roots that share a basename")
	}
}

func TestSnapshotPath_PrintsResolvedDir(t *testing.T) {
	snapshotDirOverride = func(cwd string) (string, error) {
		return filepath.Join("/snap", filepath.Base(cwd)), nil
	}
	t.Cleanup(func() { snapshotDirOverride = nil })

	var out bytes.Buffer
	root := newRootCmd()
	root.SetOut(&out)
	root.SetArgs([]string{"snapshot", "path", "/Users/yn/work/taskgate"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := strings.TrimSpace(out.String())
	if got != "/snap/taskgate" {
		t.Errorf("expected /snap/taskgate, got %q", got)
	}
}

func TestSnapshotDelete_RemovesDirAndReportsCount(t *testing.T) {
	snapDir := t.TempDir()
	makeScript(t, snapDir, "deploy", "#!/bin/sh\necho deploy")
	makeScript(t, snapDir, "build", "#!/bin/sh\necho build")

	snapshotDirOverride = func(string) (string, error) { return snapDir, nil }
	t.Cleanup(func() { snapshotDirOverride = nil })

	var out bytes.Buffer
	root := newRootCmd()
	root.SetOut(&out)
	root.SetArgs([]string{"snapshot", "delete"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(snapDir); !os.IsNotExist(err) {
		t.Errorf("expected snapshot dir to be removed, stat err = %v", err)
	}
	if !strings.Contains(out.String(), "deleted 2 script(s)") {
		t.Errorf("expected count in output, got %q", out.String())
	}
}

func TestSnapshotDelete_NoopWhenAbsent(t *testing.T) {
	snapDir := filepath.Join(t.TempDir(), "missing")
	snapshotDirOverride = func(string) (string, error) { return snapDir, nil }
	t.Cleanup(func() { snapshotDirOverride = nil })

	var out bytes.Buffer
	root := newRootCmd()
	root.SetOut(&out)
	root.SetArgs([]string{"snapshot", "delete"})
	if err := root.Execute(); err != nil {
		t.Fatalf("expected no error for absent snapshot, got %v", err)
	}

	if !strings.Contains(out.String(), "no snapshot found") {
		t.Errorf("expected 'no snapshot found' message, got %q", out.String())
	}
}
