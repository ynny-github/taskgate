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

func depRuleCounts(fs []Finding) map[string]int {
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
	if depRuleCounts(fs)[RuleDepUnknown] != 1 {
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
	if depRuleCounts(fs)[RuleDepCycle] < 1 {
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
	if depRuleCounts(fs)[RuleDepMalformed] != 1 {
		t.Fatalf("want 1 dep-malformed, got %v", fs)
	}
}

func TestDetectDeps_NotExecutable(t *testing.T) {
	dir := t.TempDir()
	dep := discoveredTask(t, dir, "shared", "build", "#!/bin/sh\n# ---\n# ---\n")
	if err := os.Chmod(dep.absPath, 0o644); err != nil {
		t.Fatal(err)
	}
	pf := map[string][]discovered{
		"human":  {discoveredTask(t, dir, "human", "deploy", "#!/bin/sh\n# ---\n# before:\n#   - build\n# ---\n")},
		"shared": {dep},
	}
	fs, err := detectDeps(show.AudienceHuman, pf)
	if err != nil {
		t.Fatal(err)
	}
	if depRuleCounts(fs)[RuleDepNotExec] != 1 {
		t.Fatalf("want 1 dep-not-exec, got %v", fs)
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
