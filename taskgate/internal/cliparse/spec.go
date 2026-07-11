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
