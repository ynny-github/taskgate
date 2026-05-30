// taskgate/cmd/snapshot_test.go
package cmd

import (
	"os"
	"path/filepath"
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
