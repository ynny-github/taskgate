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
