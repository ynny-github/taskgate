# Task Dependency Auto-Execution Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a task declare `before`/`after` dependencies in its annotation front-matter so `taskgate run` / `taskgate ai run` execute them automatically with a per-task lifecycle.

**Architecture:** A new `taskgate/internal/taskgraph` package owns graph construction (with cycle / unknown / non-executable / malformed detection) and lifecycle execution (recursive, deduplicated, immediate-`after`-on-success). A `Resolver` seam lets `run` (human+shared), `ai run` (snapshot + freshness), and `validate` (static lint) reuse the same graph logic. Annotation parsing is extended with a `ParseDeps` accessor that leaves the existing `summary`/`body` parsing untouched.

**Tech Stack:** Go, `gopkg.in/yaml.v3`, cobra (CLI), Ginkgo/Gomega (e2e suites under `tests/e2e/`).

Design reference: [`docs/superpowers/specs/2026-07-11-task-dependency-lifecycle-design.md`](../specs/2026-07-11-task-dependency-lifecycle-design.md).

## Global Constraints

- Module path: `github.com/ynny-github/taskgate`. Internal packages import as `github.com/ynny-github/taskgate/taskgate/internal/<pkg>`.
- Dependency names are `run`-style logical names (bare `build` or slash-nested `deploy/prod`); never filesystem paths.
- Only the **root target** (the task named on the CLI) receives the user-supplied arguments. All dependencies run with no arguments.
- Execution is **sequential and deterministic**; no parallelism, no timestamp/caching.
- `before`/`after` are **not** best-effort (unlike `summary`/`body`): a present-but-malformed value makes `run`/`ai run` refuse to execute anything and `validate` emit a finding.
- Child processes always receive `TASKGATE_PROJECT_ROOT` when a project root is found, exactly as the current `run`/`ai run` do.
- Conventional Commits for every commit (see `.claude/rules/git-commit.md`). Do **not** `git add` the stray `*.stdout`/`*.stderr` files in the repo root; add only the files each step names.
- Run the sandboxed test command as `go test ./...` from the repo root; `go` runs on the host per the sandbox routing rules.

## Shared Interfaces (defined in Tasks 1–3, referenced everywhere)

```go
// package annotation
type Deps struct {
	Before []string
	After  []string
}
func ParseDeps(r io.Reader) (Deps, *Diagnostic, error)

// package taskgraph
type ResolveErrorKind int
const (
	ResolveUnknown ResolveErrorKind = iota
	ResolveNotExecutable
	ResolveStale
)
type ResolveError struct {
	Name   string
	Kind   ResolveErrorKind
	Detail string
}
func (e *ResolveError) Error() string

type Resolver interface {
	Resolve(name string) (path string, err error)
}

type Runner func(path string, args []string) (exitCode int, err error)

type Node struct {
	Name   string
	Path   string
	Before []*Node
	After  []*Node
}
type Graph struct{ Root *Node }

type CycleError struct{ Name, Path string }
type MalformedDepsError struct{ Name, Path, Reason string }

func Build(target string, r Resolver) (*Graph, error)
func Execute(g *Graph, rootArgs []string, run Runner) int
```

---

### Task 1: `annotation.ParseDeps` — parse `before`/`after`

Extend the annotation parser to expose `before`/`after` without disturbing the existing `summary`/`body` path. Extract the envelope scan so both accessors share it.

**Files:**
- Modify: `taskgate/internal/annotation/annotation.go`
- Test: `taskgate/internal/annotation/deps_test.go` (create)

**Interfaces:**
- Consumes: existing `Diagnostic`, `SupportedPrefixes`, envelope helpers.
- Produces: `type Deps struct { Before, After []string }` and `func ParseDeps(r io.Reader) (Deps, *Diagnostic, error)`.

- [ ] **Step 1: Write the failing tests**

Create `taskgate/internal/annotation/deps_test.go`:

```go
package annotation

import (
	"strings"
	"testing"
)

func TestParseDeps_ListsParsed(t *testing.T) {
	src := "#!/bin/sh\n# ---\n# summary: deploy\n# before:\n#   - build\n#   - test\n# after:\n#   - notify\n# ---\necho hi\n"
	deps, diag, err := ParseDeps(strings.NewReader(src))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diag != nil {
		t.Fatalf("unexpected diagnostic: %s", diag.Reason)
	}
	if got := strings.Join(deps.Before, ","); got != "build,test" {
		t.Errorf("before = %q, want build,test", got)
	}
	if got := strings.Join(deps.After, ","); got != "notify" {
		t.Errorf("after = %q, want notify", got)
	}
}

func TestParseDeps_AbsentIsEmpty(t *testing.T) {
	src := "#!/bin/sh\n# ---\n# summary: deploy\n# ---\necho hi\n"
	deps, diag, err := ParseDeps(strings.NewReader(src))
	if err != nil || diag != nil {
		t.Fatalf("err=%v diag=%v", err, diag)
	}
	if len(deps.Before) != 0 || len(deps.After) != 0 {
		t.Errorf("expected empty deps, got %+v", deps)
	}
}

func TestParseDeps_MalformedScalar(t *testing.T) {
	src := "#!/bin/sh\n# ---\n# before: build\n# ---\necho hi\n"
	_, diag, err := ParseDeps(strings.NewReader(src))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diag == nil {
		t.Fatal("expected a diagnostic for a scalar before, got nil")
	}
	if !strings.Contains(diag.Reason, "before") {
		t.Errorf("diagnostic %q should mention before", diag.Reason)
	}
}

func TestParseDeps_NoEnvelopeIsEmpty(t *testing.T) {
	deps, diag, err := ParseDeps(strings.NewReader("#!/bin/sh\necho hi\n"))
	if err != nil || diag != nil {
		t.Fatalf("err=%v diag=%v", err, diag)
	}
	if len(deps.Before) != 0 || len(deps.After) != 0 {
		t.Errorf("expected empty deps, got %+v", deps)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./taskgate/internal/annotation/ -run TestParseDeps -v`
Expected: FAIL — `undefined: ParseDeps`.

- [ ] **Step 3: Refactor the scan and add `ParseDeps`**

In `taskgate/internal/annotation/annotation.go`, add `"fmt"` to imports if not present, then extract the envelope scan and add the deps accessor. Replace the body of `parseCore` so it delegates the scan to a new `scanEnvelope`, and append the new code:

```go
// scanEnvelope reads r, locates the front-matter envelope (skipping a leading
// shebang), and returns the inner YAML bytes with comment prefixes stripped.
// A nil error with envelopeFound=false means no envelope was present. A
// non-nil *Diagnostic reports an unterminated envelope.
func scanEnvelope(r io.Reader) (yamlBytes []byte, envelopeFound bool, diag *Diagnostic, err error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	lines := make([]string, 0, 32)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, false, nil, err
	}

	start := 0
	if start < len(lines) && strings.HasPrefix(lines[start], "#!") {
		start++
	}
	openIdx, prefix := findOpener(lines, start)
	if openIdx < 0 {
		return nil, false, nil, nil
	}
	closeIdx := findCloser(lines, openIdx+1, prefix)
	if closeIdx < 0 {
		return nil, true, &Diagnostic{Reason: "unterminated annotation envelope"}, nil
	}
	var buf bytes.Buffer
	for _, line := range lines[openIdx+1 : closeIdx] {
		buf.WriteString(stripPrefix(line, prefix))
		buf.WriteByte('\n')
	}
	return buf.Bytes(), true, nil, nil
}

// Deps is the before/after dependency lists extracted from a task's
// annotation envelope. A zero value means "no dependencies declared".
type Deps struct {
	Before []string
	After  []string
}

// ParseDeps extracts the before/after lists. Unlike summary/body, a present
// but malformed list (scalar, mapping, or non-string element) yields a
// *Diagnostic so run/validate can refuse rather than silently drop a
// prerequisite. Absent keys and an absent envelope yield an empty Deps.
func ParseDeps(r io.Reader) (Deps, *Diagnostic, error) {
	yamlBytes, found, diag, err := scanEnvelope(r)
	if err != nil {
		return Deps{}, nil, err
	}
	if !found || diag != nil {
		return Deps{}, diag, nil
	}
	var raw struct {
		Before yaml.Node `yaml:"before"`
		After  yaml.Node `yaml:"after"`
	}
	if err := yaml.Unmarshal(yamlBytes, &raw); err != nil {
		return Deps{}, &Diagnostic{Reason: "malformed YAML in annotation: " + err.Error()}, nil
	}
	before, d := decodeNameList("before", raw.Before)
	if d != nil {
		return Deps{}, d, nil
	}
	after, d := decodeNameList("after", raw.After)
	if d != nil {
		return Deps{}, d, nil
	}
	return Deps{Before: before, After: after}, nil, nil
}

// decodeNameList converts a YAML node into a []string of task names. A zero
// (absent) node yields nil. Anything that is not a sequence of scalars yields
// a *Diagnostic naming the offending key.
func decodeNameList(key string, node yaml.Node) ([]string, *Diagnostic) {
	if node.Kind == 0 {
		return nil, nil // absent
	}
	if node.Kind != yaml.SequenceNode {
		return nil, &Diagnostic{Reason: fmt.Sprintf("%s must be a list of task names", key)}
	}
	out := make([]string, 0, len(node.Content))
	for _, item := range node.Content {
		if item.Kind != yaml.ScalarNode || item.Value == "" {
			return nil, &Diagnostic{Reason: fmt.Sprintf("%s must be a list of task names", key)}
		}
		out = append(out, item.Value)
	}
	return out, nil
}
```

Then update `parseCore` to reuse `scanEnvelope` instead of scanning inline:

```go
func parseCore(r io.Reader) (AnnotationBlock, *Diagnostic, error) {
	yamlBytes, found, diag, err := scanEnvelope(r)
	if err != nil {
		return AnnotationBlock{}, nil, err
	}
	if !found || diag != nil {
		return AnnotationBlock{}, diag, nil
	}
	var doc annotationDoc
	if err := yaml.Unmarshal(yamlBytes, &doc); err != nil {
		return AnnotationBlock{}, &Diagnostic{Reason: "malformed YAML in annotation: " + err.Error()}, nil
	}
	block := AnnotationBlock{
		Summary: strings.TrimRight(doc.Summary, " \t\r\n"),
		Body:    strings.TrimRight(doc.Body, " \t\r\n"),
	}
	if strings.Contains(block.Summary, "\n") {
		return block, &Diagnostic{Reason: "summary must be a single line"}, nil
	}
	return block, nil, nil
}
```

- [ ] **Step 4: Run the annotation tests**

Run: `go test ./taskgate/internal/annotation/ -v`
Expected: PASS (new `TestParseDeps*` plus all existing tests still green — the refactor must not regress `Parse`/`ParseStrict`).

- [ ] **Step 5: Commit**

```bash
git add taskgate/internal/annotation/annotation.go taskgate/internal/annotation/deps_test.go
git commit -m "feat(annotation): parse before/after dependency lists"
```

---

### Task 2: `taskgraph.Build` — resolve, detect cycles/unknown/non-exec/malformed

Create the package with the graph builder. `Build` resolves the target and every reachable dependency, parses each file's `before`/`after`, deduplicates by resolved path, and reports the first structural error.

**Files:**
- Create: `taskgate/internal/taskgraph/taskgraph.go`
- Create: `taskgate/internal/taskgraph/build.go`
- Test: `taskgate/internal/taskgraph/build_test.go`

**Interfaces:**
- Consumes: `annotation.ParseDeps` (Task 1).
- Produces: `Resolver`, `ResolveError`, `ResolveErrorKind`, `Node`, `Graph`, `CycleError`, `MalformedDepsError`, `func Build(target string, r Resolver) (*Graph, error)`.

- [ ] **Step 1: Write the failing tests**

Create `taskgate/internal/taskgraph/build_test.go`:

```go
package taskgraph

import (
	"os"
	"path/filepath"
	"testing"
)

// mapResolver resolves names to files written under a temp dir.
type mapResolver struct{ m map[string]string }

func (r mapResolver) Resolve(name string) (string, error) {
	p, ok := r.m[name]
	if !ok {
		return "", &ResolveError{Name: name, Kind: ResolveUnknown, Detail: "not found"}
	}
	return p, nil
}

func writeTask(t *testing.T, dir, name, deps string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	body := "#!/bin/sh\n# ---\n" + deps + "# ---\necho " + name + "\n"
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestBuild_LinearChain(t *testing.T) {
	dir := t.TempDir()
	r := mapResolver{m: map[string]string{
		"deploy": writeTask(t, dir, "deploy", "# before:\n#   - build\n"),
		"build":  writeTask(t, dir, "build", ""),
	}}
	g, err := Build("deploy", r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(g.Root.Before) != 1 || g.Root.Before[0].Name != "build" {
		t.Fatalf("expected deploy before=[build], got %+v", g.Root.Before)
	}
}

func TestBuild_DiamondSharesNode(t *testing.T) {
	dir := t.TempDir()
	r := mapResolver{m: map[string]string{
		"a": writeTask(t, dir, "a", "# before:\n#   - b\n#   - c\n"),
		"b": writeTask(t, dir, "b", "# before:\n#   - d\n"),
		"c": writeTask(t, dir, "c", "# before:\n#   - d\n"),
		"d": writeTask(t, dir, "d", ""),
	}}
	g, err := Build("a", r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// b.Before[0] and c.Before[0] must be the SAME *Node (dedup by path).
	if g.Root.Before[0].Before[0] != g.Root.Before[1].Before[0] {
		t.Fatal("expected d to be a shared node pointer")
	}
}

func TestBuild_CycleDetected(t *testing.T) {
	dir := t.TempDir()
	r := mapResolver{m: map[string]string{
		"a": writeTask(t, dir, "a", "# before:\n#   - b\n"),
		"b": writeTask(t, dir, "b", "# before:\n#   - a\n"),
	}}
	_, err := Build("a", r)
	if _, ok := err.(*CycleError); !ok {
		t.Fatalf("expected *CycleError, got %v", err)
	}
}

func TestBuild_UnknownReference(t *testing.T) {
	dir := t.TempDir()
	r := mapResolver{m: map[string]string{
		"a": writeTask(t, dir, "a", "# before:\n#   - ghost\n"),
	}}
	_, err := Build("a", r)
	re, ok := err.(*ResolveError)
	if !ok || re.Kind != ResolveUnknown {
		t.Fatalf("expected ResolveUnknown, got %v", err)
	}
}

func TestBuild_MalformedDeps(t *testing.T) {
	dir := t.TempDir()
	r := mapResolver{m: map[string]string{
		"a": writeTask(t, dir, "a", "# before: build\n"),
	}}
	_, err := Build("a", r)
	if _, ok := err.(*MalformedDepsError); !ok {
		t.Fatalf("expected *MalformedDepsError, got %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./taskgate/internal/taskgraph/ -v`
Expected: FAIL — package/types undefined.

- [ ] **Step 3: Write the types and builder**

Create `taskgate/internal/taskgraph/taskgraph.go`:

```go
// Package taskgraph builds and executes a task's before/after dependency
// lifecycle. Build resolves the reachable graph and detects structural errors
// (unknown reference, non-executable, malformed deps, cycle); Execute runs the
// graph with recursive, deduplicated, immediate-after-on-success semantics.
package taskgraph

import "fmt"

// ResolveErrorKind classifies why a name failed to resolve.
type ResolveErrorKind int

const (
	// ResolveUnknown means the name matched no task in the audience view.
	ResolveUnknown ResolveErrorKind = iota
	// ResolveNotExecutable means the file exists but lacks an execute bit.
	ResolveNotExecutable
	// ResolveStale means an ai-run snapshot is out of date vs. its source.
	ResolveStale
)

// ResolveError is returned by a Resolver when a name cannot be turned into a
// runnable path.
type ResolveError struct {
	Name   string
	Kind   ResolveErrorKind
	Detail string
}

func (e *ResolveError) Error() string {
	return fmt.Sprintf("dependency %q: %s", e.Name, e.Detail)
}

// Resolver maps a run-style task name to an absolute executable path.
type Resolver interface {
	Resolve(name string) (path string, err error)
}

// Runner executes the task at path with args and returns its process exit code
// (0 = success). A non-nil error signals a spawn failure (treated as failure).
type Runner func(path string, args []string) (exitCode int, err error)

// Node is one task in the dependency graph. Nodes are deduplicated by Path, so
// a task reached through multiple edges is the same pointer.
type Node struct {
	Name   string
	Path   string
	Before []*Node
	After  []*Node
}

// Graph is the dependency graph rooted at the CLI target.
type Graph struct{ Root *Node }

// CycleError reports a dependency cycle discovered during Build.
type CycleError struct{ Name, Path string }

func (e *CycleError) Error() string {
	return fmt.Sprintf("dependency cycle detected at %q (%s)", e.Name, e.Path)
}

// MalformedDepsError reports a present-but-invalid before/after list.
type MalformedDepsError struct{ Name, Path, Reason string }

func (e *MalformedDepsError) Error() string {
	return fmt.Sprintf("task %q (%s): %s", e.Name, e.Path, e.Reason)
}
```

Create `taskgate/internal/taskgraph/build.go`:

```go
package taskgraph

import (
	"bytes"
	"os"

	"github.com/ynny-github/taskgate/taskgate/internal/annotation"
)

// Build resolves target and every reachable before/after dependency into a
// deduplicated graph. It returns the first structural error encountered:
// *ResolveError (unknown / non-executable / stale), *MalformedDepsError, or
// *CycleError.
func Build(target string, r Resolver) (*Graph, error) {
	b := &builder{r: r, byPath: map[string]*Node{}, onStack: map[string]bool{}}
	root, err := b.node(target)
	if err != nil {
		return nil, err
	}
	return &Graph{Root: root}, nil
}

type builder struct {
	r       Resolver
	byPath  map[string]*Node
	onStack map[string]bool
}

func (b *builder) node(name string) (*Node, error) {
	path, err := b.r.Resolve(name)
	if err != nil {
		return nil, err
	}
	if n, ok := b.byPath[path]; ok {
		return n, nil // dedup: already fully built
	}
	if b.onStack[path] {
		return nil, &CycleError{Name: name, Path: path}
	}
	b.onStack[path] = true
	defer delete(b.onStack, path)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	deps, diag, err := annotation.ParseDeps(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if diag != nil {
		return nil, &MalformedDepsError{Name: name, Path: path, Reason: diag.Reason}
	}

	n := &Node{Name: name, Path: path}
	for _, dep := range deps.Before {
		child, err := b.node(dep)
		if err != nil {
			return nil, err
		}
		n.Before = append(n.Before, child)
	}
	for _, dep := range deps.After {
		child, err := b.node(dep)
		if err != nil {
			return nil, err
		}
		n.After = append(n.After, child)
	}
	b.byPath[path] = n
	return n, nil
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./taskgate/internal/taskgraph/ -v`
Expected: PASS (all five `TestBuild_*`).

- [ ] **Step 5: Commit**

```bash
git add taskgate/internal/taskgraph/taskgraph.go taskgate/internal/taskgraph/build.go taskgate/internal/taskgraph/build_test.go
git commit -m "feat(taskgraph): build dependency graph with cycle detection"
```

---

### Task 3: `taskgraph.Execute` — lifecycle, dedup, immediate after, exit code

Add the executor: run each node's `before` (recursively), then its body, then — only on success — its `after`. Deduplicate by node pointer; return the first failing exit code in execution order.

**Files:**
- Create: `taskgate/internal/taskgraph/execute.go`
- Test: `taskgate/internal/taskgraph/execute_test.go`

**Interfaces:**
- Consumes: `Graph`, `Node`, `Runner` (Task 2).
- Produces: `func Execute(g *Graph, rootArgs []string, run Runner) int`.

- [ ] **Step 1: Write the failing tests**

Create `taskgate/internal/taskgraph/execute_test.go`:

```go
package taskgraph

import (
	"strings"
	"testing"
)

// recorder builds a Runner that appends each executed path's basename to a log
// and returns a preset exit code per name.
func recorder(log *[]string, fail map[string]int) Runner {
	return func(path string, args []string) (int, error) {
		name := path[strings.LastIndex(path, "/")+1:]
		*log = append(*log, name)
		if code, ok := fail[name]; ok {
			return code, nil
		}
		return 0, nil
	}
}

func n(name string) *Node { return &Node{Name: name, Path: "/t/" + name} }

func TestExecute_ImmediateAfterOrder(t *testing.T) {
	// deploy(before=[build], after=[notify]); build(after=[clean])
	build := n("build")
	build.After = []*Node{n("clean")}
	deploy := n("deploy")
	deploy.Before = []*Node{build}
	deploy.After = []*Node{n("notify")}

	var log []string
	code := Execute(&Graph{Root: deploy}, nil, recorder(&log, nil))
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	got := strings.Join(log, ",")
	if got != "build,clean,deploy,notify" {
		t.Fatalf("order = %q, want build,clean,deploy,notify", got)
	}
}

func TestExecute_DedupRunsOnce(t *testing.T) {
	d := n("d")
	b := n("b")
	b.Before = []*Node{d}
	c := n("c")
	c.Before = []*Node{d}
	a := n("a")
	a.Before = []*Node{b, c}

	var log []string
	Execute(&Graph{Root: a}, nil, recorder(&log, nil))
	count := 0
	for _, x := range log {
		if x == "d" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("d ran %d times, want 1 (log=%v)", count, log)
	}
}

func TestExecute_BeforeFailSkipsBodyAndAfter(t *testing.T) {
	deploy := n("deploy")
	deploy.Before = []*Node{n("build")}
	deploy.After = []*Node{n("notify")}

	var log []string
	code := Execute(&Graph{Root: deploy}, nil, recorder(&log, map[string]int{"build": 3}))
	if code != 3 {
		t.Fatalf("exit = %d, want 3", code)
	}
	if strings.Contains(strings.Join(log, ","), "deploy") || strings.Contains(strings.Join(log, ","), "notify") {
		t.Fatalf("deploy/notify must not run; log=%v", log)
	}
}

func TestExecute_BodyFailSkipsOwnAfter(t *testing.T) {
	deploy := n("deploy")
	deploy.After = []*Node{n("notify")}

	var log []string
	code := Execute(&Graph{Root: deploy}, nil, recorder(&log, map[string]int{"deploy": 1}))
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if strings.Contains(strings.Join(log, ","), "notify") {
		t.Fatalf("notify must be skipped; log=%v", log)
	}
}

func TestExecute_RootReceivesArgs(t *testing.T) {
	root := n("deploy")
	root.Before = []*Node{n("build")}
	var gotArgs []string
	run := func(path string, args []string) (int, error) {
		if strings.HasSuffix(path, "/deploy") {
			gotArgs = args
		} else if len(args) != 0 {
			t.Errorf("dependency %s got args %v, want none", path, args)
		}
		return 0, nil
	}
	Execute(&Graph{Root: root}, []string{"--env", "prod"}, run)
	if strings.Join(gotArgs, " ") != "--env prod" {
		t.Fatalf("root args = %v, want [--env prod]", gotArgs)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./taskgate/internal/taskgraph/ -run TestExecute -v`
Expected: FAIL — `undefined: Execute`.

- [ ] **Step 3: Write the executor**

Create `taskgate/internal/taskgraph/execute.go`:

```go
package taskgraph

// Execute runs the graph. For each node: run all before-deps (recursively),
// then the node's body, then — only if the body succeeded — all after-deps
// (recursively). Nodes are deduplicated by pointer, so a shared node's body
// runs at most once. Only the root node receives rootArgs. Returns the exit
// code of the first task that fails in execution order, or 0 if none fails.
func Execute(g *Graph, rootArgs []string, run Runner) int {
	e := &executor{run: run, done: map[*Node]int{}}
	e.visit(g.Root, rootArgs, true)
	return e.firstFail
}

type executor struct {
	run       Runner
	done      map[*Node]int // node -> exit code (present iff visited)
	firstFail int
}

func (e *executor) visit(node *Node, rootArgs []string, isRoot bool) int {
	if code, ok := e.done[node]; ok {
		return code // dedup
	}
	for _, dep := range node.Before {
		if code := e.visit(dep, nil, false); code != 0 {
			e.done[node] = code
			return code // skip body + after; short-circuit remaining before
		}
	}
	var args []string
	if isRoot {
		args = rootArgs
	}
	code, err := e.run(node.Path, args)
	if err != nil && code == 0 {
		code = 1 // spawn failure counts as failure
	}
	if code != 0 {
		e.done[node] = code
		if e.firstFail == 0 {
			e.firstFail = code
		}
		return code // skip after
	}
	e.done[node] = 0
	for _, dep := range node.After {
		e.visit(dep, nil, false) // an after-dep failure is recorded via firstFail
	}
	return 0
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./taskgate/internal/taskgraph/ -v`
Expected: PASS (all `TestBuild_*` and `TestExecute_*`).

- [ ] **Step 5: Commit**

```bash
git add taskgate/internal/taskgraph/execute.go taskgate/internal/taskgraph/execute_test.go
git commit -m "feat(taskgraph): execute before/after lifecycle with dedup"
```

---

### Task 4: Wire `taskgate run` to the graph

Replace the single-task exec in `runTask` with a graph build + execute over a human+shared resolver. Introduce an `exitError` so a non-zero task exit propagates as the process exit code without an extra `taskgate:` line.

**Files:**
- Modify: `taskgate/cmd/run.go`
- Modify: `taskgate/cmd/exec.go` (map `*exitError` to its code)
- Test: `taskgate/cmd/run_test.go` (add graph tests)

**Interfaces:**
- Consumes: `taskgraph.Build`, `taskgraph.Execute`, `taskgraph.ResolveError`, existing `detectProjectRoot`.
- Produces: `humanResolver`, `taskEnv(root)`, `type exitError struct{ code int }`.

- [ ] **Step 1: Write the failing tests**

Add to `taskgate/cmd/run_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./taskgate/cmd/ -run TestRun_ -v`
Expected: FAIL — `deploy` currently runs without `build`; order/exit assertions fail.

- [ ] **Step 3: Rewire `runTask` and add the resolver/runner**

Rewrite `runTask` in `taskgate/cmd/run.go` and add helpers. Keep `detectProjectRoot`. `resolveHumanTask` stays (the resolver reuses it and classifies the error):

```go
func runTask(cmd *cobra.Command, args []string) error {
	taskName := args[0]
	scriptArgs := args[1:]

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot determine working directory: %w", err)
	}
	root := detectProjectRoot(cwd)

	res := humanResolver{root: root}
	g, err := taskgraph.Build(taskName, res)
	if err != nil {
		return err
	}

	env := taskEnv(root)
	runner := func(path string, a []string) (int, error) {
		c := exec.Command(path, a...)
		c.Stdout = cmd.OutOrStdout()
		c.Stderr = cmd.ErrOrStderr()
		c.Stdin = os.Stdin
		c.Env = env
		if err := c.Run(); err != nil {
			var ee *exec.ExitError
			if errors.As(err, &ee) {
				return ee.ExitCode(), nil
			}
			return 0, err
		}
		return 0, nil
	}
	if code := taskgraph.Execute(g, scriptArgs, runner); code != 0 {
		return &exitError{code: code}
	}
	return nil
}

// humanResolver resolves dependency names across .taskgate/human then
// .taskgate/shared, classifying failures for taskgraph.
type humanResolver struct{ root string }

func (r humanResolver) Resolve(name string) (string, error) {
	if r.root == "" {
		return "", &taskgraph.ResolveError{Name: name, Kind: taskgraph.ResolveUnknown,
			Detail: "not found in .taskgate/human/ or .taskgate/shared/"}
	}
	for _, subdir := range []string{"human", "shared"} {
		path := filepath.Join(r.root, ".taskgate", subdir, name)
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&0o111 == 0 {
			return "", &taskgraph.ResolveError{Name: name, Kind: taskgraph.ResolveNotExecutable,
				Detail: "is not executable"}
		}
		return path, nil
	}
	return "", &taskgraph.ResolveError{Name: name, Kind: taskgraph.ResolveUnknown,
		Detail: "not found in .taskgate/human/ or .taskgate/shared/"}
}

// taskEnv returns the child environment with TASKGATE_PROJECT_ROOT managed.
func taskEnv(root string) []string {
	env := make([]string, 0, len(os.Environ())+1)
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "TASKGATE_PROJECT_ROOT=") {
			env = append(env, e)
		}
	}
	if root != "" {
		env = append(env, "TASKGATE_PROJECT_ROOT="+root)
	}
	return env
}

// exitError carries a child task's exit code up to Run without printing an
// extra diagnostic line (the child already wrote its own output).
type exitError struct{ code int }

func (e *exitError) Error() string { return fmt.Sprintf("task exited with code %d", e.code) }
```

Update the import block of `run.go` to add `"errors"` and `"github.com/ynny-github/taskgate/taskgate/internal/taskgraph"`. Delete the now-unused single-task exec body but **keep** `resolveHumanTask` (used by `run_test.go`'s existing tests) and `detectProjectRoot`.

- [ ] **Step 4: Map `exitError` in `Run`**

In `taskgate/cmd/exec.go`, add a case before the generic branch:

```go
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		var codeErr *exitError
		if errors.As(err, &codeErr) {
			return codeErr.code
		}
		var showErr *show.ExitError
		if errors.As(err, &showErr) {
			return showErr.Code
		}
```

- [ ] **Step 5: Run the tests**

Run: `go test ./taskgate/cmd/ -run TestRun -v`
Expected: PASS (new graph tests plus existing `TestResolveHumanTask_*`).

- [ ] **Step 6: Commit**

```bash
git add taskgate/cmd/run.go taskgate/cmd/exec.go taskgate/cmd/run_test.go
git commit -m "feat(run): auto-execute before/after dependencies"
```

---

### Task 5: Wire `taskgate ai run` to the graph (snapshot + freshness)

Give `runAITask` the same graph treatment with a snapshot resolver that resolves inside the snapshot dir and freshness-checks each dependency against its `.taskgate/{ai,shared}/` source.

**Files:**
- Modify: `taskgate/cmd/ai.go`
- Test: `taskgate/cmd/ai_test.go` (add graph tests using `snapshotDirOverride`)

**Interfaces:**
- Consumes: `taskgraph.Build/Execute/ResolveError`, existing `resolveAITask`, `checkSnapshotFresh`, `detectProjectRoot`, `taskEnv` (Task 4).
- Produces: `snapshotResolver`.

- [ ] **Step 1: Write the failing tests**

Inspect the existing `taskgate/cmd/ai_test.go` to reuse its `snapshotDirOverride` setup and task-writing helpers, then add:

```go
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
```

Add this helper near the top of `ai_test.go` if an equivalent does not already exist:

```go
func writeExec(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./taskgate/cmd/ -run TestAIRun -v`
Expected: FAIL — `deploy` runs alone; freshness of the dependency is not checked.

- [ ] **Step 3: Rewire `runAITask` and add the snapshot resolver**

Rewrite `runAITask` in `taskgate/cmd/ai.go`:

```go
func runAITask(cmd *cobra.Command, args []string) error {
	taskName := args[0]
	scriptArgs := args[1:]

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot determine working directory: %w", err)
	}
	dirFn := snapshotDirFn
	if snapshotDirOverride != nil {
		dirFn = snapshotDirOverride
	}
	dir, err := dirFn(cwd)
	if err != nil {
		return err
	}
	root := detectProjectRoot(cwd)

	res := snapshotResolver{snapshotDir: dir, root: root}
	g, err := taskgraph.Build(taskName, res)
	if err != nil {
		return err
	}

	env := taskEnv(root)
	runner := func(path string, a []string) (int, error) {
		c := exec.Command(path, a...)
		c.Stdout = cmd.OutOrStdout()
		c.Stderr = cmd.ErrOrStderr()
		c.Stdin = os.Stdin
		c.Env = env
		if err := c.Run(); err != nil {
			var ee *exec.ExitError
			if errors.As(err, &ee) {
				return ee.ExitCode(), nil
			}
			return 0, err
		}
		return 0, nil
	}
	if code := taskgraph.Execute(g, scriptArgs, runner); code != 0 {
		return &exitError{code: code}
	}
	return nil
}

// snapshotResolver resolves dependency names inside the snapshot dir and
// freshness-checks each against its .taskgate/{ai,shared}/ source.
type snapshotResolver struct {
	snapshotDir string
	root        string
}

func (r snapshotResolver) Resolve(name string) (string, error) {
	path, err := resolveAITask(r.snapshotDir, name)
	if err != nil {
		return "", &taskgraph.ResolveError{Name: name, Kind: taskgraph.ResolveUnknown, Detail: err.Error()}
	}
	if err := checkSnapshotFresh(r.root, name, path); err != nil {
		return "", &taskgraph.ResolveError{Name: name, Kind: taskgraph.ResolveStale, Detail: err.Error()}
	}
	return path, nil
}
```

Update the `ai.go` import block: add `"errors"` and `"github.com/ynny-github/taskgate/taskgate/internal/taskgraph"`. The `bytes`, `crypto/sha256`, `encoding/hex` imports remain (still used by snapshot helpers). Keep `resolveAITask` and `checkSnapshotFresh` unchanged.

Note: `checkSnapshotFresh` prints its own error in the old flow; now the resolver returns it as a `ResolveError` and `Build` surfaces it, so `Run` prints `taskgate: ...out of date...` once. That satisfies the "out of date" assertion. Remove the old inline `fmt.Fprintln(cmd.ErrOrStderr(), err.Error())` freshness block from the previous `runAITask` body (it no longer exists after the rewrite).

- [ ] **Step 4: Run the tests**

Run: `go test ./taskgate/cmd/ -run TestAI -v`
Expected: PASS (new `TestAIRun_*` plus existing ai tests).

- [ ] **Step 5: Commit**

```bash
git add taskgate/cmd/ai.go taskgate/cmd/ai_test.go
git commit -m "feat(ai-run): auto-execute dependencies with snapshot freshness"
```

---

### Task 6: `validate` — static dependency findings

Add static detection of unknown references, non-executable dependency targets, malformed `before`/`after`, and cycles across a bucket set, integrated into the existing validate pipeline for both audiences.

**Files:**
- Modify: `taskgate/internal/validate/finding.go` (new rule constants)
- Create: `taskgate/internal/validate/deps.go`
- Modify: `taskgate/internal/validate/validate.go` (call `detectDeps`)
- Test: `taskgate/internal/validate/deps_test.go`

**Interfaces:**
- Consumes: `discovered`, `perFiles`, `annotation.ParseDeps`, `show.Audience`.
- Produces: `func detectDeps(audience show.Audience, perFiles map[string][]discovered) ([]Finding, error)`, rule constants `RuleDepUnknown`, `RuleDepNotExec`, `RuleDepMalformed`, `RuleDepCycle`.

- [ ] **Step 1: Write the failing tests**

Create `taskgate/internal/validate/deps_test.go`:

```go
package validate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ynny-github/taskgate/taskgate/internal/show"
)

func discoveredTask(t *testing.T, dir, bucket, rel, content string) discovered {
	t.Helper()
	abs := filepath.Join(dir, ".taskgate", bucket, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	return discovered{absPath: abs, displayPath: bucketDisplayPath(bucket, rel), logicalName: rel}
}

func rulesOf(fs []Finding) map[string]int {
	m := map[string]int{}
	for _, f := range fs {
		m[f.Rule]++
	}
	return m
}

func TestDetectDeps_Unknown(t *testing.T) {
	dir := t.TempDir()
	pf := map[string][]discovered{
		"human":  {discoveredTask(t, dir, "human", "deploy", "#!/bin/sh\n# ---\n# before:\n#   - ghost\n# ---\n")},
		"shared": {},
	}
	fs, err := detectDeps(show.AudienceHuman, pf)
	if err != nil {
		t.Fatal(err)
	}
	if rulesOf(fs)[RuleDepUnknown] != 1 {
		t.Fatalf("want 1 dep-unknown, got %v", fs)
	}
}

func TestDetectDeps_Cycle(t *testing.T) {
	dir := t.TempDir()
	pf := map[string][]discovered{
		"human": {
			discoveredTask(t, dir, "human", "a", "#!/bin/sh\n# ---\n# before:\n#   - b\n# ---\n"),
			discoveredTask(t, dir, "human", "b", "#!/bin/sh\n# ---\n# before:\n#   - a\n# ---\n"),
		},
		"shared": {},
	}
	fs, err := detectDeps(show.AudienceHuman, pf)
	if err != nil {
		t.Fatal(err)
	}
	if rulesOf(fs)[RuleDepCycle] < 1 {
		t.Fatalf("want a dep-cycle finding, got %v", fs)
	}
}

func TestDetectDeps_Malformed(t *testing.T) {
	dir := t.TempDir()
	pf := map[string][]discovered{
		"human":  {discoveredTask(t, dir, "human", "a", "#!/bin/sh\n# ---\n# before: b\n# ---\n")},
		"shared": {},
	}
	fs, err := detectDeps(show.AudienceHuman, pf)
	if err != nil {
		t.Fatal(err)
	}
	if rulesOf(fs)[RuleDepMalformed] != 1 {
		t.Fatalf("want 1 dep-malformed, got %v", fs)
	}
}

func TestDetectDeps_Clean(t *testing.T) {
	dir := t.TempDir()
	pf := map[string][]discovered{
		"human":  {discoveredTask(t, dir, "human", "deploy", "#!/bin/sh\n# ---\n# before:\n#   - build\n# ---\n")},
		"shared": {discoveredTask(t, dir, "shared", "build", "#!/bin/sh\n# ---\n# ---\n")},
	}
	fs, err := detectDeps(show.AudienceHuman, pf)
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 0 {
		t.Fatalf("want no findings, got %v", fs)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./taskgate/internal/validate/ -run TestDetectDeps -v`
Expected: FAIL — `undefined: detectDeps` and the rule constants.

- [ ] **Step 3: Add rule constants**

In `taskgate/internal/validate/finding.go`, extend the const block:

```go
const (
	RuleExecBit      = "exec-bit"
	RuleShebang      = "shebang"
	RuleAnnotation   = "annotation"
	RuleCollision    = "collision"
	RuleDepUnknown   = "dep-unknown"
	RuleDepNotExec   = "dep-not-exec"
	RuleDepMalformed = "dep-malformed"
	RuleDepCycle     = "dep-cycle"
)
```

- [ ] **Step 4: Write `detectDeps`**

Create `taskgate/internal/validate/deps.go`:

```go
package validate

import (
	"bytes"
	"fmt"
	"os"
	"sort"

	"github.com/ynny-github/taskgate/taskgate/internal/annotation"
	"github.com/ynny-github/taskgate/taskgate/internal/show"
)

// depTask is a resolved task in the audience view: its file plus its declared
// edges (before ++ after), used for reference/cycle checks.
type depTask struct {
	d     discovered
	edges []string
}

// detectDeps statically checks before/after across the audience's view
// (bucket ++ shared, bucket-first). It reports malformed lists, unknown
// references, non-executable dependency targets, and cycles.
func detectDeps(audience show.Audience, perFiles map[string][]discovered) ([]Finding, error) {
	bucket := "human"
	if audience == show.AudienceAI {
		bucket = "ai"
	}

	// Merged resolution map: logical name -> discovered, bucket wins over shared.
	resolve := map[string]discovered{}
	for _, d := range perFiles["shared"] {
		resolve[d.logicalName] = d
	}
	for _, d := range perFiles[bucket] {
		resolve[d.logicalName] = d
	}

	var findings []Finding
	tasks := map[string]depTask{}
	for _, name := range sortedKeys(resolve) {
		d := resolve[name]
		data, err := os.ReadFile(d.absPath)
		if err != nil {
			return nil, err
		}
		deps, diag, err := annotation.ParseDeps(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		if diag != nil {
			findings = append(findings, Finding{
				Rule: RuleDepMalformed, Path: d.displayPath, Message: diag.Reason, logical: d.logicalName,
			})
			continue // cannot trust this task's edges
		}
		var edges []string
		for _, ref := range append(append([]string{}, deps.Before...), deps.After...) {
			target, ok := resolve[ref]
			if !ok {
				findings = append(findings, Finding{
					Rule: RuleDepUnknown, Path: d.displayPath,
					Message: fmt.Sprintf("dependency %q not found in the audience view", ref),
					logical: d.logicalName,
				})
				continue
			}
			if info, err := os.Stat(target.absPath); err == nil && info.Mode()&0o111 == 0 {
				findings = append(findings, Finding{
					Rule: RuleDepNotExec, Path: d.displayPath,
					Message: fmt.Sprintf("dependency %q is not executable", ref),
					logical: d.logicalName,
				})
				continue
			}
			edges = append(edges, ref)
		}
		tasks[name] = depTask{d: d, edges: edges}
	}

	findings = append(findings, detectCycles(tasks)...)
	return findings, nil
}

// detectCycles runs DFS coloring over the resolved edges and emits one
// dep-cycle finding per task that participates in a back-edge.
func detectCycles(tasks map[string]depTask) []Finding {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := map[string]int{}
	var out []Finding
	var seen = map[string]bool{}

	var dfs func(name string)
	dfs = func(name string) {
		color[name] = gray
		for _, next := range tasks[name].edges {
			switch color[next] {
			case gray:
				if !seen[next] {
					seen[next] = true
					out = append(out, Finding{
						Rule: RuleDepCycle, Path: tasks[next].d.displayPath,
						Message: "task participates in a dependency cycle", logical: tasks[next].d.logicalName,
					})
				}
			case white:
				dfs(next)
			}
		}
		color[name] = black
	}
	for _, name := range sortedTaskKeys(tasks) {
		if color[name] == white {
			dfs(name)
		}
	}
	return out
}

func sortedKeys(m map[string]discovered) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedTaskKeys(m map[string]depTask) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
```

- [ ] **Step 5: Call `detectDeps` from `Run`**

In `taskgate/internal/validate/validate.go`, after the `detectCollisions` line (`findings = append(findings, detectCollisions(perSlots)...)`), add:

```go
	depFindings, err := detectDeps(audience, perFiles)
	if err != nil {
		return show.ExitGeneric, err
	}
	findings = append(findings, depFindings...)
```

- [ ] **Step 6: Run the tests**

Run: `go test ./taskgate/internal/validate/ -v`
Expected: PASS (new `TestDetectDeps_*` plus all existing validate tests — the renderers already print `Path: Rule: Message` generically, so no renderer change is needed).

- [ ] **Step 7: Commit**

```bash
git add taskgate/internal/validate/finding.go taskgate/internal/validate/deps.go taskgate/internal/validate/validate.go taskgate/internal/validate/deps_test.go
git commit -m "feat(validate): report unknown/cyclic/malformed dependencies"
```

---

### Task 7: e2e — `taskgate run` dependency scenarios

Add a Ginkgo e2e suite that builds a real binary and asserts dependency ordering, dedup, failure, and cycle behavior end-to-end.

**Files:**
- Create: `tests/e2e/run/run_suite_test.go`
- Create: `tests/e2e/run/deps_test.go`
- Modify: `tests/e2e/testutil/workspace.go` (add `WriteDependentTask` + `ReadFile` helpers)

**Interfaces:**
- Consumes: `testutil.Workspace` (`New`, `Run`, `WriteFile`), the compiled binary pattern from `show_suite_test.go`.
- Produces: `RunBinary` (suite-local), `Workspace.WriteDependentTask`, `Workspace.ReadFile`.

- [ ] **Step 1: Add workspace helpers**

Append to `tests/e2e/testutil/workspace.go`:

```go
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
```

- [ ] **Step 2: Write the suite bootstrap**

Create `tests/e2e/run/run_suite_test.go` (mirrors `show_suite_test.go`):

```go
package run_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var RunBinary string

func TestRun(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Run Dependency Suite")
}

var _ = BeforeSuite(func() {
	tmpDir, err := os.MkdirTemp("", "taskgate-bin-")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(func() { os.RemoveAll(tmpDir) })

	cwd, err := os.Getwd()
	Expect(err).NotTo(HaveOccurred())
	repoRoot := filepath.Join(cwd, "..", "..", "..") // tests/e2e/run -> repo root

	binary := filepath.Join(tmpDir, "taskgate")
	cmd := exec.Command("go", "build", "-o", binary, "./taskgate")
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "go build failed: %s", output)

	RunBinary = binary
})
```

- [ ] **Step 3: Write the failing scenarios**

Create `tests/e2e/run/deps_test.go`:

```go
package run_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/ynny-github/taskgate/tests/e2e/testutil"
)

var _ = Describe("taskgate run: before/after dependency lifecycle", func() {
	var ws *testutil.Workspace

	BeforeEach(func() { ws = testutil.New(GinkgoT().TempDir(), RunBinary) })

	It("runs before deps, then the target, then after deps (immediate order)", func() {
		ws.WriteDependentTask(".taskgate/human/build", "build", nil, []string{"clean"})
		ws.WriteDependentTask(".taskgate/human/clean", "clean", nil, nil)
		ws.WriteDependentTask(".taskgate/human/notify", "notify", nil, nil)
		ws.WriteDependentTask(".taskgate/human/deploy", "deploy", []string{"build"}, []string{"notify"})

		out := ws.Run("run", "deploy")
		Expect(out.ExitCode).To(Equal(0))
		Expect(strings.Fields(ws.ReadFile("order.txt"))).To(Equal([]string{"build", "clean", "deploy", "notify"}))
	})

	It("runs a diamond-shared dependency exactly once", func() {
		ws.WriteDependentTask(".taskgate/human/d", "d", nil, nil)
		ws.WriteDependentTask(".taskgate/human/b", "b", []string{"d"}, nil)
		ws.WriteDependentTask(".taskgate/human/c", "c", []string{"d"}, nil)
		ws.WriteDependentTask(".taskgate/human/a", "a", []string{"b", "c"}, nil)

		out := ws.Run("run", "a")
		Expect(out.ExitCode).To(Equal(0))
		count := 0
		for _, f := range strings.Fields(ws.ReadFile("order.txt")) {
			if f == "d" {
				count++
			}
		}
		Expect(count).To(Equal(1))
	})

	It("aborts the target when a before dependency fails", func() {
		ws.WriteFile(".taskgate/human/build",
			"#!/bin/sh\necho build >> \""+ws.Root+"/order.txt\"\nexit 5\n", true)
		ws.WriteDependentTask(".taskgate/human/deploy", "deploy", []string{"build"}, []string{"notify"})
		ws.WriteDependentTask(".taskgate/human/notify", "notify", nil, nil)

		out := ws.Run("run", "deploy")
		Expect(out.ExitCode).To(Equal(5))
		Expect(strings.Fields(ws.ReadFile("order.txt"))).To(Equal([]string{"build"}))
	})

	It("errors on a dependency cycle without running anything", func() {
		ws.WriteDependentTask(".taskgate/human/a", "a", []string{"b"}, nil)
		ws.WriteDependentTask(".taskgate/human/b", "b", []string{"a"}, nil)

		out := ws.Run("run", "a")
		Expect(out.ExitCode).NotTo(Equal(0))
		Expect(out.Stderr).To(ContainSubstring("cycle"))
		Expect(ws.ReadFile("order.txt")).To(BeEmpty())
	})
})
```

- [ ] **Step 4: Run to verify red, then green**

Run: `go test ./tests/e2e/run/ -v`
Expected: PASS once Tasks 1–4 are in place. If run before Task 4, the ordering assertions fail (dependencies not executed) — confirming the suite exercises the feature.

- [ ] **Step 5: Commit**

```bash
git add tests/e2e/run/ tests/e2e/testutil/workspace.go
git commit -m "test(run): e2e coverage for dependency lifecycle"
```

---

### Task 8: e2e — `taskgate ai run` snapshot dependency scenario

Add an e2e scenario proving `ai run` executes a dependency from the snapshot and blocks on a stale dependency. `ai run` reads the snapshot dir from `XDG_STATE_HOME`, so the suite sets it.

**Files:**
- Create: `tests/e2e/airun/airun_suite_test.go`
- Create: `tests/e2e/airun/deps_test.go`

**Interfaces:**
- Consumes: `testutil.Workspace`, the binary-build pattern. `ai run` resolves its snapshot dir under `$XDG_STATE_HOME/taskgate/snapshots/<hash>`; the suite installs the snapshot via `taskgate snapshot install` so paths and hashing match production.

- [ ] **Step 1: Write the suite bootstrap**

Create `tests/e2e/airun/airun_suite_test.go` — identical to Task 7's `run_suite_test.go` but with `package airun_test`, `func TestAIRun`, suite name `"AI Run Dependency Suite"`, `repoRoot := filepath.Join(cwd, "..", "..", "..")`, and exported binary var `AIRunBinary`.

- [ ] **Step 2: Write the scenario**

Create `tests/e2e/airun/deps_test.go`:

```go
package airun_test

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/ynny-github/taskgate/tests/e2e/testutil"
)

var _ = Describe("taskgate ai run: dependency lifecycle from snapshot", func() {
	var ws *testutil.Workspace
	var stateHome string

	BeforeEach(func() {
		root := GinkgoT().TempDir()
		ws = testutil.New(root, AIRunBinary)
		stateHome = filepath.Join(root, "state")
		Expect(os.MkdirAll(stateHome, 0o755)).To(Succeed())
	})

	runWithState := func(args ...string) testutil.Result {
		// testutil.Run does not set env; shell out here with XDG_STATE_HOME.
		return ws.RunEnv([]string{"XDG_STATE_HOME=" + stateHome}, args...)
	}

	It("runs a before dependency from the installed snapshot", func() {
		ws.WriteDependentTask(".taskgate/ai/build", "build", nil, nil)
		ws.WriteDependentTask(".taskgate/ai/deploy", "deploy", []string{"build"}, nil)

		Expect(runWithState("snapshot", "install").ExitCode).To(Equal(0))
		out := runWithState("ai", "run", "deploy")
		Expect(out.ExitCode).To(Equal(0))
		Expect(strings.Fields(ws.ReadFile("order.txt"))).To(Equal([]string{"build", "deploy"}))
	})

	It("blocks when a dependency's snapshot is stale", func() {
		ws.WriteDependentTask(".taskgate/ai/build", "build", nil, nil)
		ws.WriteDependentTask(".taskgate/ai/deploy", "deploy", []string{"build"}, nil)
		Expect(runWithState("snapshot", "install").ExitCode).To(Equal(0))

		// Mutate the source so build's snapshot is now out of date.
		ws.WriteDependentTask(".taskgate/ai/build", "build2", nil, nil)

		out := runWithState("ai", "run", "deploy")
		Expect(out.ExitCode).NotTo(Equal(0))
		Expect(out.Stderr).To(ContainSubstring("out of date"))
	})
})
```

- [ ] **Step 3: Add `RunEnv` to the workspace helper**

Append to `tests/e2e/testutil/workspace.go`:

```go
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
```

(Confirm `snapshot install` reads `XDG_STATE_HOME`; `snapshotDirFn` in `ai.go` shows it does. If `snapshot install`'s CLI verb differs, adjust the args to match `taskgate snapshot --help`.)

- [ ] **Step 4: Run to verify green**

Run: `go test ./tests/e2e/airun/ -v`
Expected: PASS. If `snapshot install` requires interactive approval, replace it with the non-interactive install path shown by `taskgate snapshot --help` and keep the freshness assertion.

- [ ] **Step 5: Commit**

```bash
git add tests/e2e/airun/ tests/e2e/testutil/workspace.go
git commit -m "test(ai-run): e2e coverage for snapshot dependency freshness"
```

---

### Task 9: Documentation — ADR, glossary, requirements

Record the dependency model and its notable decisions so the spec's prose lands in the durable docs.

**Files:**
- Create: `docs/show/adr/0005-task-dependency-lifecycle.md`
- Modify: `docs/show/glossary.md`
- Modify: `docs/show/adr/README.md` (index the new ADR)

- [ ] **Step 1: Write the ADR**

Create `docs/show/adr/0005-task-dependency-lifecycle.md` following the format of `0004-recursive-browse-and-executable-filter.md`. Cover: the `before`/`after` front-matter keys; the per-task lifecycle (`before → body → after-on-success`); recursive traversal with dedup by physical path; the **immediate-`after` ordering consequence** (a dependency's `after` runs before a later dependent — worked example `build → clean → deploy → notify`); target-only argument passing; the strictness divergence from FR-009 for malformed `before`/`after`; audience-scoped resolution (`run` = human+shared, `ai run` = snapshot+freshness); and detection at both run time and `validate` time.

- [ ] **Step 2: Extend the glossary**

Add entries to `docs/show/glossary.md` for: **before dependency**, **after dependency**, **task lifecycle**, **root target**, **dependency graph** — using the wording from the design's Vocabulary section.

- [ ] **Step 3: Index the ADR**

Add a bullet for ADR-0005 to `docs/show/adr/README.md` matching the existing entries' style.

- [ ] **Step 4: Verify the whole suite is green**

Run: `go test ./...`
Expected: PASS across all packages and e2e suites.

- [ ] **Step 5: Commit**

```bash
git add docs/show/adr/0005-task-dependency-lifecycle.md docs/show/glossary.md docs/show/adr/README.md
git commit -m "docs(deps): document task dependency lifecycle model"
```

---

## Self-Review

**Spec coverage:**
- Front-matter `before`/`after` (§1) → Task 1.
- Immediate-`after`, recursive, dedup execution model + exit code (§2) → Task 3 (+ e2e Task 7).
- Audience-specific resolution, `run` vs `ai run` snapshot freshness (§3) → Tasks 4, 5 (+ e2e 7, 8).
- Run-time detection of cycle/unknown/non-exec/malformed (§4) → Task 2 (surfaced via Tasks 4, 5).
- `validate` static detection (§4) → Task 6.
- Package structure `Resolver`/`Build`/`Execute` (§5) → Tasks 2, 3.
- Testing (§6) → Tasks 1–8.
- Docs: ADR, glossary, requirements (§7) → Task 9.

**Placeholder scan:** No TBD/TODO; every code step carries complete code. The one runtime caveat (exact `snapshot install` verb in Task 8) is flagged with a concrete fallback rather than left vague.

**Type consistency:** `Resolver.Resolve`, `Runner`, `Node{Name,Path,Before,After}`, `Graph{Root}`, `Build`, `Execute`, `ResolveError{Name,Kind,Detail}`, `CycleError`, `MalformedDepsError`, `annotation.Deps`, `annotation.ParseDeps`, and validate rule constants are named identically across the tasks that define and consume them. `exitError`/`taskEnv` defined in Task 4 are reused in Task 5.
