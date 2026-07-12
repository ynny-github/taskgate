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
