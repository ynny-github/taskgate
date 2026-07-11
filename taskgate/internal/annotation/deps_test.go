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

func TestParseDeps_NonStringElement(t *testing.T) {
	src := "#!/bin/sh\n# ---\n# before:\n#   - 123\n# ---\necho hi\n"
	_, diag, err := ParseDeps(strings.NewReader(src))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diag == nil {
		t.Fatal("expected a diagnostic for a non-string element, got nil")
	}
	if !strings.Contains(diag.Reason, "before") {
		t.Errorf("diagnostic %q should mention before", diag.Reason)
	}
}
