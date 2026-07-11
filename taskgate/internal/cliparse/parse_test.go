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
