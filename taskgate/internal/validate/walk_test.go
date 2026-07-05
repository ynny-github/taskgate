package validate

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func mkTask(t *testing.T, ws, bucket, rel, content string) {
	t.Helper()
	p := filepath.Join(ws, bucket, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverBucket_FindsNestedTasksAndIndex(t *testing.T) {
	ws := filepath.Join(t.TempDir(), ".taskgate")
	mkTask(t, ws, "shared", "build", "#!/bin/sh\n")
	mkTask(t, ws, "shared", "deploy/prod", "#!/bin/sh\n")
	mkTask(t, ws, "shared", "deploy/_index", "---\nsummary: Deploy.\n---\n")
	mkTask(t, ws, "shared", ".gitkeep", "")

	files, slots, err := discoverBucket(ws, "shared")
	if err != nil {
		t.Fatal(err)
	}

	var logical []string
	sawIndex := false
	for _, f := range files {
		logical = append(logical, f.logicalName)
		if f.isIndex && f.logicalName == "deploy" {
			sawIndex = true
		}
	}
	sort.Strings(logical)
	// build, deploy/prod (tasks) + deploy (the _index's logical name). .gitkeep skipped.
	want := []string{"build", "deploy", "deploy/prod"}
	if len(logical) != len(want) {
		t.Fatalf("logical names = %v, want %v", logical, want)
	}
	for i := range want {
		if logical[i] != want[i] {
			t.Fatalf("logical names = %v, want %v", logical, want)
		}
	}
	if !sawIndex {
		t.Errorf("expected _index discovered with logical name 'deploy'")
	}
	// slots: task files + subdirectories, never _index.
	if _, ok := slots["build"]; !ok {
		t.Errorf("expected slot for build")
	}
	if _, ok := slots["deploy"]; !ok {
		t.Errorf("expected slot for deploy directory")
	}
	if _, ok := slots["deploy/prod"]; !ok {
		t.Errorf("expected slot for deploy/prod")
	}
	if _, ok := slots["deploy/_index"]; ok {
		t.Errorf("_index must not be a collision slot")
	}
}

func TestDetectCollisions_SharedVsHumanAndAI(t *testing.T) {
	slots := map[string]map[string]string{
		"shared": {"build": ".taskgate/shared/build", "test": ".taskgate/shared/test"},
		"human":  {"build": ".taskgate/human/build"},
		"ai":     {"test": ".taskgate/ai/test"},
	}
	got := detectCollisions(slots)
	byName := map[string][]string{}
	for _, f := range got {
		if f.Rule != RuleCollision {
			t.Fatalf("unexpected rule %q", f.Rule)
		}
		byName[f.Name] = f.Paths
	}
	if _, ok := byName["build"]; !ok {
		t.Errorf("expected shared×human collision on build, got %+v", got)
	}
	if _, ok := byName["test"]; !ok {
		t.Errorf("expected shared×ai collision on test, got %+v", got)
	}
}

func TestDetectCollisions_HumanVsAiIsNotACollision(t *testing.T) {
	slots := map[string]map[string]string{
		"shared": {},
		"human":  {"build": ".taskgate/human/build"},
		"ai":     {"build": ".taskgate/ai/build"},
	}
	if got := detectCollisions(slots); len(got) != 0 {
		t.Fatalf("human×ai must not collide, got %+v", got)
	}
}
