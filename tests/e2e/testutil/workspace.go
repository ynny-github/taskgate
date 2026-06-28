// Package testutil holds shared helpers for the taskgate E2E suite.
package testutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Result is the captured outcome of one taskgate invocation.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Workspace is a scratch directory with a .taskgate/ tree for one scenario.
type Workspace struct {
	Root   string
	binary string
}

// New binds a fresh Workspace to a pre-created tmp dir and the test binary.
// Callers typically pass GinkgoT().TempDir() or testing.T.TempDir().
func New(tmpDir, binary string) *Workspace {
	return &Workspace{Root: tmpDir, binary: binary}
}

// WriteFile creates parent dirs as needed and writes content.
func (w *Workspace) WriteFile(relpath, content string, executable bool) {
	p := filepath.Join(w.Root, relpath)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		panic(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		panic(err)
	}
	if executable {
		if err := os.Chmod(p, 0o755); err != nil {
			panic(err)
		}
	}
}

// WriteAnnotatedTask writes an sh task with the YAML front-matter annotation
// used across .taskgate/ task files (mirrors tests/conftest.py).
func (w *Workspace) WriteAnnotatedTask(relpath, summary, body string) {
	lines := []string{"#!/bin/sh", "# ---"}
	if summary != "" {
		lines = append(lines, "# summary: "+summary)
	}
	if body != "" {
		lines = append(lines, "# body: |")
		for _, line := range strings.Split(body, "\n") {
			lines = append(lines, "#   "+line)
		}
	}
	lines = append(lines, "# ---", "echo hi", "")
	w.WriteFile(relpath, strings.Join(lines, "\n"), true)
}

// WriteBareTask writes a shebang-only sh script with no annotation.
func (w *Workspace) WriteBareTask(relpath string) {
	w.WriteFile(relpath, "#!/bin/sh\necho hi\n", true)
}

// WriteIndex writes a non-executable _index with the YAML annotation envelope.
// prefix is typically "# " (use empty for bare YAML _index).
func (w *Workspace) WriteIndex(relpath, summary, body, prefix string) {
	lines := []string{prefix + "---"}
	if summary != "" {
		lines = append(lines, prefix+"summary: "+summary)
	}
	if body != "" {
		lines = append(lines, prefix+"body: |")
		for _, line := range strings.Split(body, "\n") {
			lines = append(lines, prefix+"  "+line)
		}
	}
	lines = append(lines, prefix+"---", "")
	w.WriteFile(relpath, strings.Join(lines, "\n"), false)
}

// WriteRunnableIndex writes an executable _index with shebang + annotation.
func (w *Workspace) WriteRunnableIndex(relpath, summary, body string) {
	lines := []string{"#!/bin/sh", "# ---"}
	if summary != "" {
		lines = append(lines, "# summary: "+summary)
	}
	if body != "" {
		lines = append(lines, "# body: |")
		for _, line := range strings.Split(body, "\n") {
			lines = append(lines, "#   "+line)
		}
	}
	lines = append(lines, "# ---", `echo "_index can also run"`, "")
	w.WriteFile(relpath, strings.Join(lines, "\n"), true)
}

// WriteMalformedIndex writes an _index with a known-broken YAML envelope.
func (w *Workspace) WriteMalformedIndex(relpath string) {
	w.WriteFile(relpath, "# ---\n# summary: [unclosed_array\n# ---\n", false)
}

// WriteLeadingCommentsTask writes a sh task with a shellcheck pragma and copyright
// before the YAML annotation envelope.
func (w *Workspace) WriteLeadingCommentsTask(relpath, summary string) {
	content := "#!/bin/sh\n" +
		"# shellcheck disable=SC2086\n" +
		"# Copyright (c) 2026 Example Corp.\n" +
		"# ---\n" +
		"# summary: " + summary + "\n" +
		"# ---\n" +
		"echo build\n"
	w.WriteFile(relpath, content, true)
}

// MakeUnreadable chmods the file to 0o000.
func (w *Workspace) MakeUnreadable(relpath string) {
	p := filepath.Join(w.Root, relpath)
	if err := os.Chmod(p, 0o000); err != nil {
		panic(err)
	}
}

// Symlink creates a symlink at relpath pointing to target.
func (w *Workspace) Symlink(relpath, target string) {
	link := filepath.Join(w.Root, relpath)
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		panic(err)
	}
	if err := os.Symlink(target, link); err != nil {
		panic(err)
	}
}

// WriteManyBareTasks writes count annotated tasks under dirpath named child00, child01, ...
func (w *Workspace) WriteManyBareTasks(dirpath string, count int) {
	for i := 0; i < count; i++ {
		name := fmt.Sprintf("child%02d", i)
		w.WriteAnnotatedTask(dirpath+"/"+name, fmt.Sprintf("%02d.", i), "")
	}
}

// Run invokes the bound taskgate binary with args, capturing stdout/stderr/exit.
func (w *Workspace) Run(args ...string) Result {
	cmd := exec.Command(w.binary, args...)
	cmd.Dir = w.Root
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		code = exitErr.ExitCode()
	} else if err != nil {
		code = -1
	}
	return Result{Stdout: stdout.String(), Stderr: stderr.String(), ExitCode: code}
}
