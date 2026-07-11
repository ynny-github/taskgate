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
