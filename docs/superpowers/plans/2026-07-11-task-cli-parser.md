# Task CLI Parser Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a task declare its CLI interface (positional args + flags) in its annotation front-matter; taskgate parses and validates the invocation before running, injecting validated values as `taskgate_*` environment variables.

**Architecture:** A new `internal/cliparse` package owns the validated spec model, the argv binder, and help rendering — parallel to how `internal/taskgraph` owns dependencies. Low-level YAML extraction lives in `internal/annotation` (mirroring `ParseDeps`). `cmd/run.go` and `cmd/ai.go` parse the root target's spec between `taskgraph.Build` and `taskgraph.Execute`; `internal/validate` and `internal/show` reuse the same model for static linting and machine-readable display.

**Tech Stack:** Go, `gopkg.in/yaml.v3`, `spf13/cobra`, Ginkgo/Gomega + testscript for e2e.

## Global Constraints

- Module path prefix: `github.com/ynny-github/taskgate/taskgate/internal/...`.
- No new third-party dependencies (yaml.v3 + cobra only).
- Spec-less tasks (neither `args` nor `flags`) MUST behave exactly as today: `[args...]` forwarded to the task's argv unchanged.
- Injected env variable names are `taskgate_` + the entry name with leading `-`/`--` stripped, lowercased, every non-alphanumeric run collapsed to a single `_`.
- Exit codes: `--help` prints and exits `0`; an invocation usage error exits `2`; a malformed/invalid spec (authoring bug) exits `1`.
- Only the root target is parsed; dependencies receive no args and are never parsed.
- Conventional Commits for every commit (`.claude/rules/git-commit.md`).
- Output language for docs/comments/messages: English.

---

### Task 1: Annotation raw arg-spec parsing

Add YAML extraction of the `args`/`flags` block to the annotation package, mirroring the existing `ParseDeps`. This produces raw, un-validated declarations; semantic rules live in Task 2.

**Files:**
- Modify: `taskgate/internal/annotation/annotation.go`
- Test: `taskgate/internal/annotation/argspec_test.go` (create)

**Interfaces:**
- Consumes: the existing unexported `scanEnvelope(r) ([]byte, bool, *Diagnostic, error)` and the `Diagnostic` type in the same package.
- Produces:
  - `type RawSpec struct { Args []RawArg; Flags []RawFlag }`
  - `type RawArg struct { Name string; Help string; Choices []string; Required bool; Default *string; Variadic bool }`
  - `type RawFlag struct { Name string; Short string; Help string; Type string; Choices []string; Default *string }`
  - `func ParseArgSpec(r io.Reader) (RawSpec, *Diagnostic, error)`

- [ ] **Step 1: Write the failing test**

Create `taskgate/internal/annotation/argspec_test.go`:

```go
package annotation

import "strings"

import "testing"

func TestParseArgSpec_Absent(t *testing.T) {
	spec, diag, err := ParseArgSpec(strings.NewReader("#!/bin/sh\n# ---\n# summary: hi\n# ---\n"))
	if err != nil || diag != nil {
		t.Fatalf("unexpected diag=%v err=%v", diag, err)
	}
	if len(spec.Args) != 0 || len(spec.Flags) != 0 {
		t.Fatalf("expected empty spec, got %+v", spec)
	}
}

func TestParseArgSpec_ArgsAndFlags(t *testing.T) {
	src := strings.Join([]string{
		"#!/bin/sh",
		"# ---",
		"# args:",
		"#   - name: env",
		"#     choices: [staging, prod]",
		"#     required: true",
		"#   - name: files",
		"#     variadic: true",
		"# flags:",
		"#   - name: --tag",
		"#     default: latest",
		"#   - name: --dry-run",
		"#     short: -n",
		"#     type: bool",
		"# ---",
		"",
	}, "\n")
	spec, diag, err := ParseArgSpec(strings.NewReader(src))
	if err != nil || diag != nil {
		t.Fatalf("unexpected diag=%v err=%v", diag, err)
	}
	if len(spec.Args) != 2 || spec.Args[0].Name != "env" || !spec.Args[0].Required {
		t.Fatalf("bad args: %+v", spec.Args)
	}
	if !spec.Args[1].Variadic {
		t.Fatalf("expected files variadic: %+v", spec.Args[1])
	}
	if len(spec.Flags) != 2 || spec.Flags[0].Name != "--tag" || spec.Flags[0].Default == nil || *spec.Flags[0].Default != "latest" {
		t.Fatalf("bad flags: %+v", spec.Flags)
	}
	if spec.Flags[1].Short != "-n" || spec.Flags[1].Type != "bool" {
		t.Fatalf("bad dry-run flag: %+v", spec.Flags[1])
	}
}

func TestParseArgSpec_NotAList(t *testing.T) {
	src := "# ---\n# args: nope\n# ---\n"
	_, diag, err := ParseArgSpec(strings.NewReader(src))
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if diag == nil || !strings.Contains(diag.Reason, "args must be a list") {
		t.Fatalf("expected args-list diagnostic, got %v", diag)
	}
}

func TestParseArgSpec_MissingName(t *testing.T) {
	src := "# ---\n# args:\n#   - help: no name here\n# ---\n"
	_, diag, err := ParseArgSpec(strings.NewReader(src))
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if diag == nil || !strings.Contains(diag.Reason, "name") {
		t.Fatalf("expected missing-name diagnostic, got %v", diag)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./taskgate/internal/annotation/ -run TestParseArgSpec`
Expected: FAIL — `undefined: ParseArgSpec`.

- [ ] **Step 3: Write minimal implementation**

Append to `taskgate/internal/annotation/annotation.go` (the `fmt` and `yaml` imports already exist):

```go
// RawSpec is the args/flags CLI declaration as literally written, before
// semantic validation (which lives in internal/cliparse). A zero value means
// no spec was declared.
type RawSpec struct {
	Args  []RawArg
	Flags []RawFlag
}

// RawArg is one positional-argument declaration.
type RawArg struct {
	Name     string
	Help     string
	Choices  []string
	Required bool
	Default  *string // nil = key absent
	Variadic bool
}

// RawFlag is one flag declaration. Type is "", "string", or "bool".
type RawFlag struct {
	Name    string
	Short   string
	Help    string
	Type    string
	Choices []string
	Default *string // nil = key absent
}

// ParseArgSpec extracts the args/flags declaration. Like ParseDeps, a present
// but structurally malformed block (args/flags not a list, an entry that is not
// a mapping, or an entry missing `name`) yields a *Diagnostic so run/validate
// can refuse. Absent keys and an absent envelope yield an empty RawSpec.
func ParseArgSpec(r io.Reader) (RawSpec, *Diagnostic, error) {
	yamlBytes, found, diag, err := scanEnvelope(r)
	if err != nil {
		return RawSpec{}, nil, err
	}
	if !found || diag != nil {
		return RawSpec{}, diag, nil
	}
	var raw struct {
		Args  yaml.Node `yaml:"args"`
		Flags yaml.Node `yaml:"flags"`
	}
	if err := yaml.Unmarshal(yamlBytes, &raw); err != nil {
		return RawSpec{}, &Diagnostic{Reason: "malformed YAML in annotation: " + err.Error()}, nil
	}
	args, d := decodeArgs(raw.Args)
	if d != nil {
		return RawSpec{}, d, nil
	}
	flags, d := decodeFlags(raw.Flags)
	if d != nil {
		return RawSpec{}, d, nil
	}
	return RawSpec{Args: args, Flags: flags}, nil, nil
}

func decodeArgs(node yaml.Node) ([]RawArg, *Diagnostic) {
	if node.Kind == 0 {
		return nil, nil
	}
	if node.Kind != yaml.SequenceNode {
		return nil, &Diagnostic{Reason: "args must be a list"}
	}
	out := make([]RawArg, 0, len(node.Content))
	for _, item := range node.Content {
		if item.Kind != yaml.MappingNode {
			return nil, &Diagnostic{Reason: "each args entry must be a mapping"}
		}
		var a RawArg
		if err := item.Decode(&a); err != nil {
			return nil, &Diagnostic{Reason: "malformed args entry: " + err.Error()}
		}
		if a.Name == "" {
			return nil, &Diagnostic{Reason: "each args entry needs a name"}
		}
		out = append(out, a)
	}
	return out, nil
}

func decodeFlags(node yaml.Node) ([]RawFlag, *Diagnostic) {
	if node.Kind == 0 {
		return nil, nil
	}
	if node.Kind != yaml.SequenceNode {
		return nil, &Diagnostic{Reason: "flags must be a list"}
	}
	out := make([]RawFlag, 0, len(node.Content))
	for _, item := range node.Content {
		if item.Kind != yaml.MappingNode {
			return nil, &Diagnostic{Reason: "each flags entry must be a mapping"}
		}
		var f RawFlag
		if err := item.Decode(&f); err != nil {
			return nil, &Diagnostic{Reason: "malformed flags entry: " + err.Error()}
		}
		if f.Name == "" {
			return nil, &Diagnostic{Reason: "each flags entry needs a name"}
		}
		out = append(out, f)
	}
	return out, nil
}
```

Note: `item.Decode` binds `Default *string` to nil when the `default` key is absent, distinguishing "no default" from an empty-string default.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./taskgate/internal/annotation/ -run TestParseArgSpec`
Expected: PASS (all four subtests).

- [ ] **Step 5: Commit**

```bash
git add taskgate/internal/annotation/annotation.go taskgate/internal/annotation/argspec_test.go
git commit -m "feat(annotation): parse args/flags CLI spec declarations"
```

---

### Task 2: cliparse spec model + Compile (semantic validation, var derivation)

Create the `cliparse` package: a validated `Spec` built from `annotation.RawSpec`, with semantic checks and derived variable names.

**Files:**
- Create: `taskgate/internal/cliparse/spec.go`
- Test: `taskgate/internal/cliparse/spec_test.go`

**Interfaces:**
- Consumes: `annotation.RawSpec`, `annotation.RawArg`, `annotation.RawFlag` (Task 1).
- Produces:
  - `type Arg struct { Name, Help string; Choices []string; Required bool; Default *string; Variadic bool; Var string }`
  - `type Flag struct { Name, Short, Help string; Bool bool; Choices []string; Default *string; Var string }`
  - `type Spec struct { Args []Arg; Flags []Flag }`
  - `func Compile(raw annotation.RawSpec) (*Spec, []string)` — returns `(nil, nil)` when `raw` declares nothing; otherwise the built spec plus a slice of human-readable problem messages (empty when valid).
  - `func deriveVar(name string) string` (unexported; also used by later tasks in-package).

- [ ] **Step 1: Write the failing test**

Create `taskgate/internal/cliparse/spec_test.go`:

```go
package cliparse

import (
	"strings"
	"testing"

	"github.com/ynny-github/taskgate/taskgate/internal/annotation"
)

func ptr(s string) *string { return &s }

func TestCompile_Empty(t *testing.T) {
	spec, probs := Compile(annotation.RawSpec{})
	if spec != nil || probs != nil {
		t.Fatalf("expected nil spec/probs, got %v %v", spec, probs)
	}
}

func TestCompile_DerivesVars(t *testing.T) {
	raw := annotation.RawSpec{
		Args:  []annotation.RawArg{{Name: "env"}, {Name: "files", Variadic: true}},
		Flags: []annotation.RawFlag{{Name: "--dry-run", Type: "bool"}, {Name: "--tag"}},
	}
	spec, probs := Compile(raw)
	if len(probs) != 0 {
		t.Fatalf("unexpected problems: %v", probs)
	}
	if spec.Args[0].Var != "env" || spec.Args[1].Var != "files" {
		t.Fatalf("bad arg vars: %+v", spec.Args)
	}
	if spec.Flags[0].Var != "dry_run" || !spec.Flags[0].Bool {
		t.Fatalf("bad dry-run: %+v", spec.Flags[0])
	}
	if spec.Flags[1].Var != "tag" || spec.Flags[1].Bool {
		t.Fatalf("bad tag: %+v", spec.Flags[1])
	}
}

func TestCompile_Problems(t *testing.T) {
	cases := []struct {
		name string
		raw  annotation.RawSpec
		want string
	}{
		{"requiredWithDefault",
			annotation.RawSpec{Args: []annotation.RawArg{{Name: "a", Required: true, Default: ptr("x")}}},
			"cannot be both required and have a default"},
		{"defaultNotInChoices",
			annotation.RawSpec{Args: []annotation.RawArg{{Name: "a", Choices: []string{"x"}, Default: ptr("y")}}},
			"default \"y\" is not one of its choices"},
		{"variadicNotLast",
			annotation.RawSpec{Args: []annotation.RawArg{{Name: "a", Variadic: true}, {Name: "b"}}},
			"only the last argument may be variadic"},
		{"requiredAfterOptional",
			annotation.RawSpec{Args: []annotation.RawArg{{Name: "a"}, {Name: "b", Required: true}}},
			"required argument \"b\" cannot follow an optional argument"},
		{"flagNoDashes",
			annotation.RawSpec{Flags: []annotation.RawFlag{{Name: "tag"}}},
			"flag name \"tag\" must start with --"},
		{"badShort",
			annotation.RawSpec{Flags: []annotation.RawFlag{{Name: "--tag", Short: "-xy"}}},
			"short \"-xy\" must be a single dash and character"},
		{"boolWithDefault",
			annotation.RawSpec{Flags: []annotation.RawFlag{{Name: "--x", Type: "bool", Default: ptr("y")}}},
			"bool flag \"--x\" cannot have choices or a default"},
		{"varCollision",
			annotation.RawSpec{Args: []annotation.RawArg{{Name: "dry-run"}}, Flags: []annotation.RawFlag{{Name: "--dry-run", Type: "bool"}}},
			"both map to environment variable taskgate_dry_run"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, probs := Compile(tc.raw)
			joined := strings.Join(probs, "\n")
			if !strings.Contains(joined, tc.want) {
				t.Fatalf("want %q in problems, got %q", tc.want, joined)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./taskgate/internal/cliparse/ -run TestCompile`
Expected: FAIL — package/`Compile` undefined.

- [ ] **Step 3: Write minimal implementation**

Create `taskgate/internal/cliparse/spec.go`:

```go
// Package cliparse compiles a task's declared CLI spec (annotation.RawSpec)
// into a validated model, binds an invocation's argv against it, and renders
// help. It is the arg-spec analogue of internal/taskgraph.
package cliparse

import (
	"fmt"
	"strings"

	"github.com/ynny-github/taskgate/taskgate/internal/annotation"
)

// Arg is a validated positional argument.
type Arg struct {
	Name     string
	Help     string
	Choices  []string
	Required bool
	Default  *string
	Variadic bool
	Var      string // env-var suffix, e.g. "env" (no taskgate_ prefix)
}

// Flag is a validated flag.
type Flag struct {
	Name    string // "--dry-run"
	Short   string // "-n" or ""
	Help    string
	Bool    bool
	Choices []string
	Default *string
	Var     string
}

// Spec is a validated CLI spec for one task.
type Spec struct {
	Args  []Arg
	Flags []Flag
}

// Compile builds a validated Spec from a raw declaration. It returns (nil, nil)
// when nothing is declared. Otherwise it returns the (best-effort) spec plus a
// list of human-readable problem messages; an empty list means the spec is
// valid.
func Compile(raw annotation.RawSpec) (*Spec, []string) {
	if len(raw.Args) == 0 && len(raw.Flags) == 0 {
		return nil, nil
	}
	var probs []string
	spec := &Spec{}
	vars := map[string]string{} // var -> declaring name, for collision detection

	claim := func(v, name string) {
		if prev, ok := vars[v]; ok {
			probs = append(probs, fmt.Sprintf("%q and %q both map to environment variable taskgate_%s", prev, name, v))
			return
		}
		vars[v] = name
	}

	seenOptional := false
	for i, a := range raw.Args {
		arg := Arg{Name: a.Name, Help: a.Help, Choices: a.Choices,
			Required: a.Required, Default: a.Default, Variadic: a.Variadic, Var: deriveVar(a.Name)}
		if a.Variadic && i != len(raw.Args)-1 {
			probs = append(probs, "only the last argument may be variadic")
		}
		if a.Required && a.Default != nil {
			probs = append(probs, fmt.Sprintf("argument %q cannot be both required and have a default", a.Name))
		}
		if a.Default != nil && len(a.Choices) > 0 && !contains(a.Choices, *a.Default) {
			probs = append(probs, fmt.Sprintf("argument %q default %q is not one of its choices", a.Name, *a.Default))
		}
		optional := !a.Required
		if a.Required && seenOptional {
			probs = append(probs, fmt.Sprintf("required argument %q cannot follow an optional argument", a.Name))
		}
		if optional {
			seenOptional = true
		}
		claim(arg.Var, a.Name)
		spec.Args = append(spec.Args, arg)
	}

	for _, f := range raw.Flags {
		flag := Flag{Name: f.Name, Short: f.Short, Help: f.Help,
			Bool: f.Type == "bool", Choices: f.Choices, Default: f.Default, Var: deriveVar(f.Name)}
		if !strings.HasPrefix(f.Name, "--") {
			probs = append(probs, fmt.Sprintf("flag name %q must start with --", f.Name))
		}
		if f.Short != "" && !(len(f.Short) == 2 && f.Short[0] == '-' && f.Short[1] != '-') {
			probs = append(probs, fmt.Sprintf("flag %q short %q must be a single dash and character", f.Name, f.Short))
		}
		if f.Type != "" && f.Type != "bool" && f.Type != "string" {
			probs = append(probs, fmt.Sprintf("flag %q has unknown type %q (want bool or string)", f.Name, f.Type))
		}
		if flag.Bool && (len(f.Choices) > 0 || f.Default != nil) {
			probs = append(probs, fmt.Sprintf("bool flag %q cannot have choices or a default", f.Name))
		}
		if !flag.Bool && f.Default != nil && len(f.Choices) > 0 && !contains(f.Choices, *f.Default) {
			probs = append(probs, fmt.Sprintf("flag %q default %q is not one of its choices", f.Name, *f.Default))
		}
		claim(flag.Var, f.Name)
		spec.Flags = append(spec.Flags, flag)
	}
	return spec, probs
}

// deriveVar strips leading dashes, lowercases, and collapses each run of
// non-alphanumeric characters into a single underscore.
func deriveVar(name string) string {
	name = strings.TrimLeft(name, "-")
	var b strings.Builder
	prevUnderscore := false
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevUnderscore = false
		} else if !prevUnderscore {
			b.WriteByte('_')
			prevUnderscore = true
		}
	}
	return strings.Trim(b.String(), "_")
}

func contains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./taskgate/internal/cliparse/ -run TestCompile`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add taskgate/internal/cliparse/spec.go taskgate/internal/cliparse/spec_test.go
git commit -m "feat(cliparse): compile and validate task CLI specs"
```

---

### Task 3: cliparse.Parse (argv binding + env map)

Bind an invocation's argv against a compiled `Spec`, producing the `taskgate_*` environment map or a usage error.

**Files:**
- Create: `taskgate/internal/cliparse/parse.go`
- Test: `taskgate/internal/cliparse/parse_test.go`

**Interfaces:**
- Consumes: `*Spec`, `Arg`, `Flag` (Task 2).
- Produces:
  - `type Result struct { Help bool; Env map[string]string }` — `Env` keys carry the full `taskgate_` prefix.
  - `type UsageError struct { Reason string }` with `func (e *UsageError) Error() string`.
  - `func (s *Spec) Parse(argv []string) (Result, *UsageError)`.

- [ ] **Step 1: Write the failing test**

Create `taskgate/internal/cliparse/parse_test.go`:

```go
package cliparse

import (
	"testing"

	"github.com/ynny-github/taskgate/taskgate/internal/annotation"
)

func compile(t *testing.T, raw annotation.RawSpec) *Spec {
	t.Helper()
	spec, probs := Compile(raw)
	if len(probs) != 0 {
		t.Fatalf("compile problems: %v", probs)
	}
	return spec
}

func TestParse_HappyPath(t *testing.T) {
	spec := compile(t, annotation.RawSpec{
		Args: []annotation.RawArg{
			{Name: "env", Choices: []string{"staging", "prod"}, Required: true},
			{Name: "files", Variadic: true},
		},
		Flags: []annotation.RawFlag{
			{Name: "--dry-run", Short: "-n", Type: "bool"},
			{Name: "--tag", Default: ptr("latest")},
		},
	})
	res, uerr := spec.Parse([]string{"prod", "a.txt", "b c.txt", "-n"})
	if uerr != nil {
		t.Fatalf("unexpected usage error: %v", uerr)
	}
	want := map[string]string{
		"taskgate_env":         "prod",
		"taskgate_tag":         "latest",
		"taskgate_dry_run":     "true",
		"taskgate_files_count": "2",
		"taskgate_files_1":     "a.txt",
		"taskgate_files_2":     "b c.txt",
	}
	for k, v := range want {
		if res.Env[k] != v {
			t.Errorf("env[%s]=%q want %q", k, res.Env[k], v)
		}
	}
	if len(res.Env) != len(want) {
		t.Errorf("env has %d keys, want %d: %v", len(res.Env), len(want), res.Env)
	}
}

func TestParse_BoolFalseAndUnsetOptional(t *testing.T) {
	spec := compile(t, annotation.RawSpec{
		Args:  []annotation.RawArg{{Name: "opt"}},
		Flags: []annotation.RawFlag{{Name: "--dry-run", Type: "bool"}},
	})
	res, uerr := spec.Parse(nil)
	if uerr != nil {
		t.Fatalf("unexpected: %v", uerr)
	}
	if res.Env["taskgate_dry_run"] != "false" {
		t.Errorf("dry_run=%q want false", res.Env["taskgate_dry_run"])
	}
	if _, ok := res.Env["taskgate_opt"]; ok {
		t.Errorf("optional-without-default should be unset, got %q", res.Env["taskgate_opt"])
	}
	if res.Env["taskgate_files_count"] != "" {
		// no variadic declared; ensure no stray count
	}
}

func TestParse_Errors(t *testing.T) {
	base := annotation.RawSpec{
		Args:  []annotation.RawArg{{Name: "env", Choices: []string{"staging", "prod"}, Required: true}},
		Flags: []annotation.RawFlag{{Name: "--tag"}},
	}
	cases := []struct {
		name string
		argv []string
		want string
	}{
		{"missingRequired", nil, `missing required argument <env>`},
		{"badChoice", []string{"dev"}, `argument <env>: must be one of staging, prod`},
		{"unknownFlag", []string{"prod", "--bogus"}, `unknown flag --bogus`},
		{"flagNeedsValue", []string{"prod", "--tag"}, `flag --tag needs a value`},
		{"tooMany", []string{"prod", "extra"}, `unexpected argument "extra"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := compile(t, base)
			_, uerr := spec.Parse(tc.argv)
			if uerr == nil || uerr.Reason != tc.want {
				t.Fatalf("got %v, want reason %q", uerr, tc.want)
			}
		})
	}
}

func TestParse_HelpRequested(t *testing.T) {
	spec := compile(t, annotation.RawSpec{Flags: []annotation.RawFlag{{Name: "--tag"}}})
	res, uerr := spec.Parse([]string{"--help"})
	if uerr != nil || !res.Help {
		t.Fatalf("expected help, got res=%+v uerr=%v", res, uerr)
	}
}

func TestParse_VariadicZero(t *testing.T) {
	spec := compile(t, annotation.RawSpec{Args: []annotation.RawArg{{Name: "files", Variadic: true}}})
	res, uerr := spec.Parse(nil)
	if uerr != nil {
		t.Fatalf("unexpected: %v", uerr)
	}
	if res.Env["taskgate_files_count"] != "0" {
		t.Fatalf("count=%q want 0", res.Env["taskgate_files_count"])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./taskgate/internal/cliparse/ -run TestParse`
Expected: FAIL — `spec.Parse` undefined.

- [ ] **Step 3: Write minimal implementation**

Create `taskgate/internal/cliparse/parse.go`:

```go
package cliparse

import (
	"fmt"
	"strconv"
	"strings"
)

// Result is the outcome of binding an invocation. When Help is true the caller
// should print help and exit 0; Env is then empty.
type Result struct {
	Help bool
	Env  map[string]string
}

// UsageError is a bad invocation (missing/unknown/invalid argument). The caller
// prints Reason plus a usage line and exits 2.
type UsageError struct{ Reason string }

func (e *UsageError) Error() string { return e.Reason }

// Parse binds argv against the spec. Flags may appear in any position; the
// trailing variadic arg (if any) absorbs the remaining positionals. Recognized
// forms: --flag, --flag value, -n. --flag=value and bundled shorts are not
// supported.
func (s *Spec) Parse(argv []string) (Result, *UsageError) {
	env := map[string]string{}
	var positionals []string
	seenFlag := map[string]bool{}

	for i := 0; i < len(argv); i++ {
		tok := argv[i]
		if tok == "--help" || tok == "-h" {
			if s.findFlagByToken(tok) == nil { // reserved unless the task declares it
				return Result{Help: true}, nil
			}
		}
		if len(tok) > 1 && tok[0] == '-' {
			f := s.findFlagByToken(tok)
			if f == nil {
				return Result{}, &UsageError{Reason: "unknown flag " + tok}
			}
			if f.Bool {
				env["taskgate_"+f.Var] = "true"
				seenFlag[f.Name] = true
				continue
			}
			if i+1 >= len(argv) {
				return Result{}, &UsageError{Reason: "flag " + f.Name + " needs a value"}
			}
			i++
			if len(f.Choices) > 0 && !contains(f.Choices, argv[i]) {
				return Result{}, &UsageError{Reason: fmt.Sprintf("flag %s: must be one of %s", f.Name, strings.Join(f.Choices, ", "))}
			}
			env["taskgate_"+f.Var] = argv[i]
			seenFlag[f.Name] = true
			continue
		}
		positionals = append(positionals, tok)
	}

	// Bind positionals to declared args in order.
	pi := 0
	for _, a := range s.Args {
		if a.Variadic {
			rest := positionals[pi:]
			for j, v := range rest {
				if len(a.Choices) > 0 && !contains(a.Choices, v) {
					return Result{}, &UsageError{Reason: fmt.Sprintf("argument <%s>: must be one of %s", a.Name, strings.Join(a.Choices, ", "))}
				}
				env[fmt.Sprintf("taskgate_%s_%d", a.Var, j+1)] = v
			}
			env["taskgate_"+a.Var+"_count"] = strconv.Itoa(len(rest))
			pi = len(positionals)
			continue
		}
		if pi < len(positionals) {
			v := positionals[pi]
			pi++
			if len(a.Choices) > 0 && !contains(a.Choices, v) {
				return Result{}, &UsageError{Reason: fmt.Sprintf("argument <%s>: must be one of %s", a.Name, strings.Join(a.Choices, ", "))}
			}
			env["taskgate_"+a.Var] = v
			continue
		}
		if a.Default != nil {
			env["taskgate_"+a.Var] = *a.Default
			continue
		}
		if a.Required {
			return Result{}, &UsageError{Reason: fmt.Sprintf("missing required argument <%s>", a.Name)}
		}
		// optional without default: leave unset
	}
	if pi < len(positionals) {
		return Result{}, &UsageError{Reason: fmt.Sprintf("unexpected argument %q", positionals[pi])}
	}

	// Fill defaults / bool-false for flags not seen.
	for _, f := range s.Flags {
		if seenFlag[f.Name] {
			continue
		}
		if f.Bool {
			env["taskgate_"+f.Var] = "false"
		} else if f.Default != nil {
			env["taskgate_"+f.Var] = *f.Default
		}
	}
	return Result{Env: env}, nil
}

// findFlagByToken matches a "--long" or "-s" token to a declared flag, or nil.
func (s *Spec) findFlagByToken(tok string) *Flag {
	for i := range s.Flags {
		if s.Flags[i].Name == tok || (s.Flags[i].Short != "" && s.Flags[i].Short == tok) {
			return &s.Flags[i]
		}
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./taskgate/internal/cliparse/`
Expected: PASS (Task 2 + Task 3 tests).

- [ ] **Step 5: Commit**

```bash
git add taskgate/internal/cliparse/parse.go taskgate/internal/cliparse/parse_test.go
git commit -m "feat(cliparse): bind invocation argv into taskgate_* env"
```

---

### Task 4: cliparse help & usage-line rendering

Render `--help` text and the one-line usage string from a compiled spec.

**Files:**
- Create: `taskgate/internal/cliparse/help.go`
- Test: `taskgate/internal/cliparse/help_test.go`

**Interfaces:**
- Consumes: `*Spec` (Task 2).
- Produces:
  - `func (s *Spec) UsageLine(invocation string) string` — e.g. `Usage: taskgate run deploy [flags] <env> [files...]`.
  - `func (s *Spec) Help(invocation, summary, body string) string`.

- [ ] **Step 1: Write the failing test**

Create `taskgate/internal/cliparse/help_test.go`:

```go
package cliparse

import (
	"strings"
	"testing"

	"github.com/ynny-github/taskgate/taskgate/internal/annotation"
)

func TestUsageLine(t *testing.T) {
	spec := compile(t, annotation.RawSpec{
		Args: []annotation.RawArg{
			{Name: "env", Required: true},
			{Name: "files", Variadic: true},
		},
		Flags: []annotation.RawFlag{{Name: "--tag"}},
	})
	got := spec.UsageLine("taskgate run deploy")
	want := "Usage: taskgate run deploy [flags] <env> [files...]"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestHelp_ContainsSections(t *testing.T) {
	spec := compile(t, annotation.RawSpec{
		Args:  []annotation.RawArg{{Name: "env", Help: "Target environment", Choices: []string{"staging", "prod"}, Required: true}},
		Flags: []annotation.RawFlag{{Name: "--dry-run", Short: "-n", Type: "bool", Help: "Skip side effects"}},
	})
	out := spec.Help("taskgate run deploy", "Deploy to an environment.", "Body line.")
	for _, frag := range []string{
		"Deploy to an environment.",
		"Usage: taskgate run deploy",
		"Arguments:",
		"<env>",
		"Target environment",
		"choices: staging, prod",
		"Flags:",
		"-n, --dry-run",
		"Skip side effects",
		"-h, --help",
		"Body line.",
	} {
		if !strings.Contains(out, frag) {
			t.Errorf("help missing %q\n---\n%s", frag, out)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./taskgate/internal/cliparse/ -run 'TestUsageLine|TestHelp'`
Expected: FAIL — `UsageLine`/`Help` undefined.

- [ ] **Step 3: Write minimal implementation**

Create `taskgate/internal/cliparse/help.go`:

```go
package cliparse

import (
	"fmt"
	"strings"
)

// UsageLine renders a single-line usage synopsis.
func (s *Spec) UsageLine(invocation string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Usage: %s", invocation)
	if len(s.Flags) > 0 {
		b.WriteString(" [flags]")
	}
	for _, a := range s.Args {
		switch {
		case a.Variadic:
			fmt.Fprintf(&b, " [%s...]", a.Name)
		case a.Required:
			fmt.Fprintf(&b, " <%s>", a.Name)
		default:
			fmt.Fprintf(&b, " [%s]", a.Name)
		}
	}
	return b.String()
}

// Help renders the full --help text: summary, usage line, argument and flag
// tables, then the body.
func (s *Spec) Help(invocation, summary, body string) string {
	var b strings.Builder
	if summary != "" {
		b.WriteString(summary)
		b.WriteString("\n\n")
	}
	b.WriteString(s.UsageLine(invocation))
	b.WriteString("\n")

	if len(s.Args) > 0 {
		b.WriteString("\nArguments:\n")
		for _, a := range s.Args {
			label := "<" + a.Name + ">"
			if a.Variadic {
				label = "[" + a.Name + "...]"
			} else if !a.Required {
				label = "[" + a.Name + "]"
			}
			fmt.Fprintf(&b, "  %-14s %s%s\n", label, a.Help, annotate(a.Choices, a.Default))
		}
	}

	b.WriteString("\nFlags:\n")
	for _, f := range s.Flags {
		head := "    " + f.Name
		if f.Short != "" {
			head = f.Short + ", " + f.Name
		}
		fmt.Fprintf(&b, "  %-14s %s%s\n", head, f.Help, annotate(f.Choices, f.Default))
	}
	fmt.Fprintf(&b, "  %-14s %s\n", "-h, --help", "Show this help")

	if body != "" {
		b.WriteString("\n")
		b.WriteString(body)
		b.WriteString("\n")
	}
	return b.String()
}

func annotate(choices []string, def *string) string {
	var parts []string
	if len(choices) > 0 {
		parts = append(parts, "choices: "+strings.Join(choices, ", "))
	}
	if def != nil {
		parts = append(parts, "default: "+*def)
	}
	if len(parts) == 0 {
		return ""
	}
	return "  (" + strings.Join(parts, "; ") + ")"
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./taskgate/internal/cliparse/`
Expected: PASS (all cliparse tests).

- [ ] **Step 5: Commit**

```bash
git add taskgate/internal/cliparse/help.go taskgate/internal/cliparse/help_test.go
git commit -m "feat(cliparse): render --help and usage-line text"
```

---

### Task 5: Wire the parser into `taskgate run`

Add a shared `applyRootSpec` helper and call it in `runTask` between `Build` and `Execute`. Inject env only for the root task; forward the raw args unchanged when the task has no spec.

**Files:**
- Create: `taskgate/cmd/rootspec.go`
- Modify: `taskgate/cmd/run.go` (`runTask`, around lines 34-60)
- Test (e2e): `tests/e2e/run/cliparse_test.go`

**Interfaces:**
- Consumes: `annotation.ParseArgSpec` (Task 1), `annotation.Parse` (existing, for summary/body), `cliparse.Compile`/`Parse`/`Help`/`UsageLine` (Tasks 2-4), the existing `exitError` type (`run.go:113`), `taskgraph.Build`/`Execute` and `g.Root.Path`.
- Produces:
  - `func applyRootSpec(rootPath, invocation string, scriptArgs []string, stdout, stderr io.Writer) (adds, forwarded []string, handled bool, err error)`.
    - No spec: `adds=nil, forwarded=scriptArgs, handled=false, err=nil`.
    - `--help`: writes help to stdout; `handled=true`.
    - Usage error: writes `taskgate: <reason>` + usage line to stderr; `err=&exitError{code:2}`.
    - Malformed/invalid spec: `err` is a plain `error` (Run prints it and exits 1).
    - Valid spec + valid invocation: `adds` are `"k=v"` strings, `forwarded=nil` (empty root argv).

- [ ] **Step 1: Write the failing test**

Create `tests/e2e/run/cliparse_test.go`:

```go
package run_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/ynny-github/taskgate/tests/e2e/testutil"
)

var _ = Describe("taskgate run: CLI parser", func() {
	var ws *testutil.Workspace
	BeforeEach(func() { ws = testutil.New(GinkgoT().TempDir(), RunBinary) })

	const deploy = `#!/bin/sh
# ---
# summary: Deploy to an environment.
# args:
#   - name: env
#     choices: [staging, prod]
#     required: true
#   - name: files
#     variadic: true
# flags:
#   - name: --dry-run
#     short: -n
#     type: bool
#   - name: --tag
#     default: latest
# ---
echo "env=$taskgate_env tag=$taskgate_tag dry=$taskgate_dry_run n=$taskgate_files_count f1=$taskgate_files_1"
`

	It("injects taskgate_* env and empties argv", func() {
		ws.WriteFile(".taskgate/human/deploy", deploy, true)
		out := ws.Run("run", "deploy", "prod", "a.txt", "-n")
		Expect(out.ExitCode).To(Equal(0))
		Expect(out.Stdout).To(ContainSubstring("env=prod tag=latest dry=true n=1 f1=a.txt"))
	})

	It("rejects a bad choice with exit 2 and a usage line", func() {
		ws.WriteFile(".taskgate/human/deploy", deploy, true)
		out := ws.Run("run", "deploy", "dev")
		Expect(out.ExitCode).To(Equal(2))
		Expect(out.Stderr).To(ContainSubstring("must be one of staging, prod"))
		Expect(out.Stderr).To(ContainSubstring("Usage: taskgate run deploy"))
	})

	It("prints --help and exits 0 without running", func() {
		ws.WriteFile(".taskgate/human/deploy", deploy, true)
		out := ws.Run("run", "deploy", "--help")
		Expect(out.ExitCode).To(Equal(0))
		Expect(out.Stdout).To(ContainSubstring("Usage: taskgate run deploy"))
		Expect(out.Stdout).To(ContainSubstring("Deploy to an environment."))
	})

	It("passes args through unchanged when no spec is declared", func() {
		ws.WriteFile(".taskgate/human/raw", "#!/bin/sh\necho \"got: $1 $2\"\n", true)
		out := ws.Run("run", "raw", "one", "two")
		Expect(out.ExitCode).To(Equal(0))
		Expect(strings.TrimSpace(out.Stdout)).To(Equal("got: one two"))
	})
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tests/e2e/run/ -run CLI`
Expected: FAIL — the spec'd task receives raw argv (env vars empty), no exit-2 handling.

- [ ] **Step 3: Write the shared helper**

Create `taskgate/cmd/rootspec.go`:

```go
package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/ynny-github/taskgate/taskgate/internal/annotation"
	"github.com/ynny-github/taskgate/taskgate/internal/cliparse"
)

// applyRootSpec parses the root target's CLI spec (if any) against scriptArgs.
// See the plan's Task 5 interface block for the return-value contract.
func applyRootSpec(rootPath, invocation string, scriptArgs []string, stdout, stderr io.Writer) (adds, forwarded []string, handled bool, err error) {
	data, err := os.ReadFile(rootPath)
	if err != nil {
		return nil, nil, false, err
	}
	raw, diag, err := annotation.ParseArgSpec(bytes.NewReader(data))
	if err != nil {
		return nil, nil, false, err
	}
	if diag != nil {
		return nil, nil, false, fmt.Errorf("%s: %s", invocation, diag.Reason)
	}
	spec, probs := cliparse.Compile(raw)
	if len(probs) > 0 {
		return nil, nil, false, fmt.Errorf("%s: %s", invocation, probs[0])
	}
	if spec == nil { // no spec declared → raw passthrough
		return nil, scriptArgs, false, nil
	}
	res, uerr := spec.Parse(scriptArgs)
	if uerr != nil {
		fmt.Fprintf(stderr, "taskgate: %s\n%s\n", uerr.Reason, spec.UsageLine(invocation))
		return nil, nil, false, &exitError{code: 2}
	}
	if res.Help {
		block, _ := annotation.Parse(bytes.NewReader(data))
		fmt.Fprint(stdout, spec.Help(invocation, block.Summary, block.Body))
		return nil, nil, true, nil
	}
	for k, v := range res.Env {
		adds = append(adds, k+"="+v)
	}
	return adds, nil, false, nil
}
```

- [ ] **Step 4: Wire it into `runTask`**

In `taskgate/cmd/run.go`, replace the body from the `env := taskEnv(root)` line through the `taskgraph.Execute` call (lines ~42-61) with:

```go
	adds, forwarded, handled, err := applyRootSpec(g.Root.Path, "taskgate run "+taskName, scriptArgs, cmd.OutOrStdout(), cmd.ErrOrStderr())
	if err != nil {
		return err
	}
	if handled {
		return nil
	}

	env := taskEnv(root)
	rootEnv := append(append([]string{}, env...), adds...)
	runner := func(path string, a []string) (int, error) {
		e := env
		if path == g.Root.Path {
			e = rootEnv
		}
		c := exec.Command(path, a...)
		c.Stdout = cmd.OutOrStdout()
		c.Stderr = cmd.ErrOrStderr()
		c.Stdin = os.Stdin
		c.Env = e
		if err := c.Run(); err != nil {
			var ee *exec.ExitError
			if errors.As(err, &ee) {
				return ee.ExitCode(), nil
			}
			return 0, err
		}
		return 0, nil
	}
	if code := taskgraph.Execute(g, forwarded, runner); code != 0 {
		return &exitError{code: code}
	}
	return nil
```

(When the task has no spec, `adds` is nil so `rootEnv == env`, and `forwarded == scriptArgs` — identical to today.)

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./tests/e2e/run/ -run CLI && go build ./...`
Expected: PASS; build clean.

- [ ] **Step 6: Commit**

```bash
git add taskgate/cmd/rootspec.go taskgate/cmd/run.go tests/e2e/run/cliparse_test.go
git commit -m "feat(run): parse and validate task CLI spec before execution"
```

---

### Task 6: Wire the parser into `taskgate ai run`

Reuse `applyRootSpec` in `runAITask` with the same structure.

**Files:**
- Modify: `taskgate/cmd/ai.go` (`runAITask`, lines ~99-118)
- Test (e2e): `tests/e2e/airun/cliparse_test.go`

**Interfaces:**
- Consumes: `applyRootSpec` (Task 5), `g.Root.Path`, existing snapshot resolver/runner plumbing.

- [ ] **Step 1: Write the failing test**

Create `tests/e2e/airun/cliparse_test.go`. Mirror the run test but drive `ai run` from an installed snapshot. Follow the existing setup in `tests/e2e/airun/deps_test.go` for writing `.taskgate/ai/` tasks and running `ai run` (use `ws.Run("ai", "run", "deploy", ...)` and whatever snapshot-install step that suite already uses):

```go
package airun_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/ynny-github/taskgate/tests/e2e/testutil"
)

var _ = Describe("taskgate ai run: CLI parser", func() {
	var ws *testutil.Workspace
	BeforeEach(func() { ws = testutil.New(GinkgoT().TempDir(), RunBinary) })

	const deploy = `#!/bin/sh
# ---
# summary: Deploy.
# args:
#   - name: env
#     choices: [staging, prod]
#     required: true
# ---
echo "env=$taskgate_env"
`

	It("injects env under ai run", func() {
		ws.WriteFile(".taskgate/ai/deploy", deploy, true)
		// Install/approve the snapshot exactly as tests/e2e/airun/deps_test.go does.
		ws.Run("snapshot", "install")
		out := ws.Run("ai", "run", "deploy", "prod")
		Expect(out.ExitCode).To(Equal(0))
		Expect(out.Stdout).To(ContainSubstring("env=prod"))
	})

	It("rejects a bad choice with exit 2", func() {
		ws.WriteFile(".taskgate/ai/deploy", deploy, true)
		ws.Run("snapshot", "install")
		out := ws.Run("ai", "run", "deploy", "dev")
		Expect(out.ExitCode).To(Equal(2))
		Expect(out.Stderr).To(ContainSubstring("must be one of staging, prod"))
	})
})
```

Before writing, open `tests/e2e/airun/deps_test.go` and copy its exact snapshot-install / approval sequence — replace the `ws.Run("snapshot", "install")` placeholder above with the real steps if they differ.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tests/e2e/airun/ -run CLI`
Expected: FAIL — env not injected under `ai run`.

- [ ] **Step 3: Wire it into `runAITask`**

In `taskgate/cmd/ai.go`, replace the block from `env := taskEnv(root)` through the `taskgraph.Execute` call (lines ~99-118) with the same shape as Task 5, but with the `ai run` invocation label:

```go
	adds, forwarded, handled, err := applyRootSpec(g.Root.Path, "taskgate ai run "+taskName, scriptArgs, cmd.OutOrStdout(), cmd.ErrOrStderr())
	if err != nil {
		return err
	}
	if handled {
		return nil
	}

	env := taskEnv(root)
	rootEnv := append(append([]string{}, env...), adds...)
	runner := func(path string, a []string) (int, error) {
		e := env
		if path == g.Root.Path {
			e = rootEnv
		}
		c := exec.Command(path, a...)
		c.Stdout = cmd.OutOrStdout()
		c.Stderr = cmd.ErrOrStderr()
		c.Stdin = os.Stdin
		c.Env = e
		if err := c.Run(); err != nil {
			var ee *exec.ExitError
			if errors.As(err, &ee) {
				return ee.ExitCode(), nil
			}
			return 0, err
		}
		return 0, nil
	}
	if code := taskgraph.Execute(g, forwarded, runner); code != 0 {
		return &exitError{code: code}
	}
	return nil
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./tests/e2e/airun/ -run CLI && go build ./...`
Expected: PASS; build clean.

- [ ] **Step 5: Commit**

```bash
git add taskgate/cmd/ai.go tests/e2e/airun/cliparse_test.go
git commit -m "feat(ai-run): parse and validate task CLI spec before execution"
```

---

### Task 7: Static linting in `validate` / `ai validate`

Emit findings for malformed and semantically-invalid arg specs, mirroring `detectDeps`.

**Files:**
- Create: `taskgate/internal/validate/spec.go`
- Modify: `taskgate/internal/validate/finding.go` (add rule constants), `taskgate/internal/validate/validate.go` (call `detectSpec`, ~line 66)
- Test: `taskgate/internal/validate/spec_test.go`

**Interfaces:**
- Consumes: `annotation.ParseArgSpec` (Task 1), `cliparse.Compile` (Task 2), the existing `discovered` type, `perFiles map[string][]discovered`, `show.Audience`, and the `Finding` type.
- Produces:
  - Rule constants `RuleSpecMalformed = "spec-malformed"` and `RuleSpecInvalid = "spec-invalid"`.
  - `func detectSpec(audience show.Audience, perFiles map[string][]discovered) ([]Finding, error)`.

- [ ] **Step 1: Write the failing test**

Create `taskgate/internal/validate/spec_test.go`:

```go
package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ynny-github/taskgate/taskgate/internal/show"
)

func writeTask(t *testing.T, dir, name, body string) discovered {
	t.Helper()
	abs := filepath.Join(dir, name)
	if err := os.WriteFile(abs, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return discovered{logicalName: name, absPath: abs, displayPath: ".taskgate/human/" + name}
}

func TestDetectSpec_Invalid(t *testing.T) {
	dir := t.TempDir()
	d := writeTask(t, dir, "bad", "#!/bin/sh\n# ---\n# flags:\n#   - name: tag\n# ---\n")
	perFiles := map[string][]discovered{"human": {d}, "shared": nil}
	findings, err := detectSpec(show.AudienceHuman, perFiles)
	if err != nil {
		t.Fatal(err)
	}
	joined := ""
	for _, f := range findings {
		joined += f.Rule + ":" + f.Message + "\n"
	}
	if !strings.Contains(joined, "spec-invalid") || !strings.Contains(joined, "must start with --") {
		t.Fatalf("expected spec-invalid finding, got:\n%s", joined)
	}
}

func TestDetectSpec_Malformed(t *testing.T) {
	dir := t.TempDir()
	d := writeTask(t, dir, "bad", "#!/bin/sh\n# ---\n# args: nope\n# ---\n")
	perFiles := map[string][]discovered{"human": {d}, "shared": nil}
	findings, err := detectSpec(show.AudienceHuman, perFiles)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].Rule != RuleSpecMalformed {
		t.Fatalf("expected one spec-malformed finding, got %+v", findings)
	}
}

func TestDetectSpec_Valid(t *testing.T) {
	dir := t.TempDir()
	d := writeTask(t, dir, "ok", "#!/bin/sh\n# ---\n# flags:\n#   - name: --tag\n#     default: latest\n# ---\n")
	perFiles := map[string][]discovered{"human": {d}, "shared": nil}
	findings, err := detectSpec(show.AudienceHuman, perFiles)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %+v", findings)
	}
}
```

(Confirm the `discovered` struct field names by reading `taskgate/internal/validate/walk.go` — adjust the `writeTask` literal if the fields differ from `logicalName`/`absPath`/`displayPath`.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./taskgate/internal/validate/ -run TestDetectSpec`
Expected: FAIL — `detectSpec`, `RuleSpecInvalid`, `RuleSpecMalformed` undefined.

- [ ] **Step 3: Add rule constants and the detector**

Add to `taskgate/internal/validate/finding.go` const block:

```go
	RuleSpecMalformed = "spec-malformed"
	RuleSpecInvalid   = "spec-invalid"
```

Create `taskgate/internal/validate/spec.go`:

```go
package validate

import (
	"bytes"
	"os"

	"github.com/ynny-github/taskgate/taskgate/internal/annotation"
	"github.com/ynny-github/taskgate/taskgate/internal/cliparse"
	"github.com/ynny-github/taskgate/taskgate/internal/show"
)

// detectSpec statically lints each task's args/flags CLI declaration across the
// audience's view (bucket + shared). It reports a malformed block (bad YAML
// shape) and semantic problems (bad flag names, required+default, etc.).
func detectSpec(audience show.Audience, perFiles map[string][]discovered) ([]Finding, error) {
	bucket := "human"
	if audience == show.AudienceAI {
		bucket = "ai"
	}
	resolve := map[string]discovered{}
	for _, d := range perFiles["shared"] {
		resolve[d.logicalName] = d
	}
	for _, d := range perFiles[bucket] {
		resolve[d.logicalName] = d
	}

	var findings []Finding
	for _, name := range sortedKeys(resolve) {
		d := resolve[name]
		data, err := os.ReadFile(d.absPath)
		if err != nil {
			return nil, err
		}
		raw, diag, err := annotation.ParseArgSpec(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		if diag != nil {
			findings = append(findings, Finding{
				Rule: RuleSpecMalformed, Path: d.displayPath, Message: diag.Reason, logical: d.logicalName,
			})
			continue
		}
		_, probs := cliparse.Compile(raw)
		for _, p := range probs {
			findings = append(findings, Finding{
				Rule: RuleSpecInvalid, Path: d.displayPath, Message: p, logical: d.logicalName,
			})
		}
	}
	return findings, nil
}
```

Then in `taskgate/internal/validate/validate.go`, after the `detectDeps` block (~line 70), add:

```go
	specFindings, err := detectSpec(audience, perFiles)
	if err != nil {
		return 0, err
	}
	findings = append(findings, specFindings...)
```

(Match the surrounding error-return arity; `detectDeps`'s call site shows the exact shape.)

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./taskgate/internal/validate/`
Expected: PASS (new tests + existing).

- [ ] **Step 5: Add an e2e golden and commit**

Add one scenario to the validate e2e suite (find it via `grep -rl "ai validate" tests/e2e`) asserting that a task with `flags: [{name: tag}]` produces a `spec-invalid` finding and a non-zero exit, and a valid spec produces `"ok":true`. Then:

```bash
git add taskgate/internal/validate/spec.go taskgate/internal/validate/finding.go taskgate/internal/validate/validate.go taskgate/internal/validate/spec_test.go tests/e2e/
git commit -m "feat(validate): report malformed and invalid CLI specs"
```

---

### Task 8: Surface the spec in `ai show` and human `show`

Add `args`/`flags` to the `ai show` task JSON and a usage line to human `show`.

**Files:**
- Modify: `taskgate/internal/show/render_ai.go` (`taskEnvelope`, ~line 24), `taskgate/internal/show/show.go` (`renderAITarget` ~line 123, and the human target renderer)
- Test: extend `taskgate/internal/show/render_test.go` and/or the show e2e suite

**Interfaces:**
- Consumes: `annotation.ParseArgSpec` (Task 1), `cliparse.Compile`/`UsageLine` (Tasks 2, 4), `target.Entry` (has the task's absolute path — confirm the field name in `show.go`).
- Produces:
  - `taskEnvelope.Args []argJSON` and `taskEnvelope.Flags []flagJSON` (both `json:",omitempty"`).
  - `type argJSON struct { Name string `json:"name"`; Help string `json:"help,omitempty"`; Choices []string `json:"choices,omitempty"`; Required bool `json:"required,omitempty"`; Default *string `json:"default,omitempty"`; Variadic bool `json:"variadic,omitempty"` }`
  - `type flagJSON struct { Name string `json:"name"`; Short string `json:"short,omitempty"`; Type string `json:"type"`; Help string `json:"help,omitempty"`; Choices []string `json:"choices,omitempty"`; Default *string `json:"default,omitempty"` }`
  - `func specJSON(path string) (args []argJSON, flags []flagJSON)` — best-effort; returns nils on read/parse/compile failure.

- [ ] **Step 1: Write the failing test**

Add to `taskgate/internal/show/render_test.go` (or the show e2e suite — match the file that already asserts task JSON):

```go
func TestSpecJSON(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "deploy")
	body := "#!/bin/sh\n# ---\n# args:\n#   - name: env\n#     choices: [staging, prod]\n#     required: true\n# flags:\n#   - name: --dry-run\n#     type: bool\n# ---\n"
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	args, flags := specJSON(p)
	if len(args) != 1 || args[0].Name != "env" || !args[0].Required || len(args[0].Choices) != 2 {
		t.Fatalf("bad args: %+v", args)
	}
	if len(flags) != 1 || flags[0].Name != "--dry-run" || flags[0].Type != "bool" {
		t.Fatalf("bad flags: %+v", flags)
	}
}
```

(Add `os`, `path/filepath`, `testing` imports as needed.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./taskgate/internal/show/ -run TestSpecJSON`
Expected: FAIL — `specJSON` and the JSON types undefined.

- [ ] **Step 3: Implement the JSON helper and wire it**

Add the `argJSON`/`flagJSON` structs to `taskgate/internal/show/render_ai.go` and the two new fields to `taskEnvelope`:

```go
	Args     []argJSON  `json:"args,omitempty"`
	Flags    []flagJSON `json:"flags,omitempty"`
```

Add `specJSON` (in `render_ai.go` or a new `spec.go` in the package):

```go
func specJSON(path string) (args []argJSON, flags []flagJSON) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil
	}
	raw, diag, err := annotation.ParseArgSpec(bytes.NewReader(data))
	if err != nil || diag != nil {
		return nil, nil
	}
	spec, probs := cliparse.Compile(raw)
	if spec == nil || len(probs) > 0 {
		return nil, nil
	}
	for _, a := range spec.Args {
		args = append(args, argJSON{Name: a.Name, Help: a.Help, Choices: a.Choices,
			Required: a.Required, Default: a.Default, Variadic: a.Variadic})
	}
	for _, f := range spec.Flags {
		typ := "string"
		if f.Bool {
			typ = "bool"
		}
		flags = append(flags, flagJSON{Name: f.Name, Short: f.Short, Type: typ,
			Help: f.Help, Choices: f.Choices, Default: f.Default})
	}
	return args, flags
}
```

In `renderAITarget` (`show.go` ~line 126), populate the envelope from the target's path:

```go
		args, flags := specJSON(target.Entry.Path) // confirm the path field name
		env := taskEnvelope{
			// ...existing fields...
			Args:  args,
			Flags: flags,
		}
```

In the human target renderer, after printing the summary, print `spec.UsageLine("taskgate run " + name)` when a spec compiles cleanly (best-effort; skip on nil spec). Read the existing human render function to match its writer and formatting.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./taskgate/internal/show/ && go build ./...`
Expected: PASS; build clean.

- [ ] **Step 5: Commit**

```bash
git add taskgate/internal/show/
git commit -m "feat(show): expose task CLI spec in ai show and human show"
```

---

### Task 9: Documentation

Record the model in an ADR, glossary, requirements, and the AI usage guide.

**Files:**
- Create: `docs/show/adr/0006-task-cli-parser.md`
- Modify: `docs/show/glossary.md`, `docs/show/requirements.md`, `docs/show/adr/README.md`, `taskgate/internal/usage/guide.md`

**Interfaces:** none (docs only). This task closes the `project_ai_usage_guide_sync` obligation.

- [ ] **Step 1: Write the ADR**

Create `docs/show/adr/0006-task-cli-parser.md` following the style of `0005-task-dependency-lifecycle.md`. Record: extending the YAML annotation with `args`/`flags`; `taskgate_*` env injection (variadic → `_count` + `_1..N`); empty-argv for spec'd tasks; spec-less passthrough; exit codes (help 0 / usage 2 / invalid-spec 1); root-target-only parsing; shell completion deferred to v2.

- [ ] **Step 2: Update glossary and requirements**

Add the Vocabulary terms from the spec (arg spec, positional argument, flag, choices, injected variable) to `docs/show/glossary.md`. Add a requirements note to `docs/show/requirements.md` capturing the run-time parse/validate guarantee and the spec-less passthrough guarantee. Add the `0006` line to `docs/show/adr/README.md`.

- [ ] **Step 3: Update the AI usage guide**

Add a "Task arguments (args/flags)" section to `taskgate/internal/usage/guide.md` documenting: how to declare `args`/`flags`; that `taskgate ai run <name> [args...]` validates the invocation and injects `taskgate_<name>` variables (variadic as `taskgate_<name>_count` + `taskgate_<name>_1..N`); that `ai show <name>` reports the spec; and that a usage error exits 2. Keep the tone and format of the existing sections.

- [ ] **Step 4: Verify the guide is embedded and referenced correctly**

Run: `go test ./taskgate/cmd/ -run Usage`
Expected: PASS (the `ai_usage` test embeds `guide.md`; if it snapshots content, update its golden).

- [ ] **Step 5: Commit**

```bash
git add docs/ taskgate/internal/usage/guide.md
git commit -m "docs(cliparse): document task CLI parser model and usage"
```

---

## Final Verification

- [ ] Run the whole suite: `go test ./... && go build ./...` — all green.
- [ ] Manual smoke: create `.taskgate/human/deploy` with the spec from Task 5, run `taskgate run deploy --help`, `taskgate run deploy dev` (exit 2), `taskgate run deploy prod a.txt -n` (env injected), and a spec-less task (unchanged).
