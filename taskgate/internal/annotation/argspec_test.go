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
