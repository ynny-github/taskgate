// taskgate/cmd/init_test.go
package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitCmd_CreatesDirectories(t *testing.T) {
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
	root.SetArgs([]string{"init"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, subdir := range []string{"ai", "human", "shared"} {
		info, err := os.Stat(filepath.Join(tmp, ".taskgate", subdir))
		if err != nil {
			t.Errorf("expected .taskgate/%s/ to exist: %v", subdir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf(".taskgate/%s is not a directory", subdir)
		}
	}
}

func TestInitCmd_CreatesExecutableExamples(t *testing.T) {
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
	root.SetArgs([]string{"init"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, subdir := range []string{"ai", "human", "shared"} {
		path := filepath.Join(tmp, ".taskgate", subdir, "example")
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("expected example script in .taskgate/%s/: %v", subdir, err)
			continue
		}
		if info.Mode()&0111 == 0 {
			t.Errorf(".taskgate/%s/example is not executable", subdir)
		}
	}
}

func TestInitCmd_SkipsIfTaskgateDirExists(t *testing.T) {
	tmp := t.TempDir()
	if err := os.Mkdir(filepath.Join(tmp, ".taskgate"), 0755); err != nil {
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

	var errBuf bytes.Buffer
	root := newRootCmd()
	root.SetArgs([]string{"init"})
	root.SetErr(&errBuf)
	if err := root.Execute(); err != nil {
		t.Fatalf("expected success (skip), got error: %v", err)
	}
	if !strings.Contains(errBuf.String(), ".taskgate") {
		t.Errorf("expected stderr to mention .taskgate, got: %q", errBuf.String())
	}
}

func TestInitCmd_PrintsConfirmation(t *testing.T) {
	tmp := t.TempDir()
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
	root.SetArgs([]string{"init"})
	root.SetOut(&buf)
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), ".taskgate") {
		t.Errorf("expected stdout to mention .taskgate, got: %q", buf.String())
	}
}

func TestInitCmd_WritesClaudeMdPointer(t *testing.T) {
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
	root.SetArgs([]string{"init"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmp, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("expected CLAUDE.md to be created: %v", err)
	}
	if !strings.Contains(string(data), "taskgate ai usage") {
		t.Errorf("CLAUDE.md missing the usage pointer, got: %q", string(data))
	}
}

func TestInitCmd_EnsuresPointerEvenWhenTaskgateExists(t *testing.T) {
	tmp := t.TempDir()
	if err := os.Mkdir(filepath.Join(tmp, ".taskgate"), 0755); err != nil {
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

	root := newRootCmd()
	root.SetArgs([]string{"init"})
	if err := root.Execute(); err != nil {
		t.Fatalf("expected success even when .taskgate exists: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmp, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("expected CLAUDE.md pointer even when .taskgate exists: %v", err)
	}
	if !strings.Contains(string(data), "taskgate ai usage") {
		t.Errorf("CLAUDE.md missing the usage pointer, got: %q", string(data))
	}
}

func TestInitCmd_PointerIsIdempotent(t *testing.T) {
	tmp := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	for i := 0; i < 2; i++ {
		root := newRootCmd()
		root.SetArgs([]string{"init"})
		if err := root.Execute(); err != nil {
			t.Fatalf("run %d: unexpected error: %v", i, err)
		}
	}

	data, err := os.ReadFile(filepath.Join(tmp, "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(data), claudeMdBeginMarker); n != 1 {
		t.Errorf("expected exactly one managed block after two runs, got %d", n)
	}
}
