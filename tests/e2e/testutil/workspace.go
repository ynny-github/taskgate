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

// WriteDependentTask writes an executable sh task that appends its own name to
// <Root>/order.txt, with the given before/after dependency lists in its
// annotation. Pass nil for no dependencies of that kind.
func (w *Workspace) WriteDependentTask(relpath, name string, before, after []string) {
	lines := []string{"#!/bin/sh", "# ---"}
	appendList := func(key string, names []string) {
		if len(names) == 0 {
			return
		}
		lines = append(lines, "# "+key+":")
		for _, n := range names {
			lines = append(lines, "#   - "+n)
		}
	}
	appendList("before", before)
	appendList("after", after)
	lines = append(lines, "# ---", `echo `+name+` >> "$0.dir/order.txt"`, "")
	// Resolve order.txt relative to the workspace root via an absolute path.
	body := strings.Join(lines, "\n")
	body = strings.ReplaceAll(body, `"$0.dir/order.txt"`, `"`+filepath.Join(w.Root, "order.txt")+`"`)
	w.WriteFile(relpath, body, true)
}

// ReadFile returns the content of a workspace-relative path (empty if absent).
func (w *Workspace) ReadFile(relpath string) string {
	b, err := os.ReadFile(filepath.Join(w.Root, relpath))
	if err != nil {
		return ""
	}
	return string(b)
}

// RunEnv is Run with extra environment variables appended to the child env.
func (w *Workspace) RunEnv(extraEnv []string, args ...string) Result {
	cmd := exec.Command(w.binary, args...)
	cmd.Dir = w.Root
	cmd.Env = append(os.Environ(), extraEnv...)
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
