package taskgraph

import (
	"os"
	"path/filepath"
	"testing"
)

// mapResolver resolves names to files written under a temp dir.
type mapResolver struct{ m map[string]string }

func (r mapResolver) Resolve(name string) (string, error) {
	p, ok := r.m[name]
	if !ok {
		return "", &ResolveError{Name: name, Kind: ResolveUnknown, Detail: "not found"}
	}
	return p, nil
}

func writeTask(t *testing.T, dir, name, deps string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	body := "#!/bin/sh\n# ---\n" + deps + "# ---\necho " + name + "\n"
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestBuild_LinearChain(t *testing.T) {
	dir := t.TempDir()
	r := mapResolver{m: map[string]string{
		"deploy": writeTask(t, dir, "deploy", "# before:\n#   - build\n"),
		"build":  writeTask(t, dir, "build", ""),
	}}
	g, err := Build("deploy", r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(g.Root.Before) != 1 || g.Root.Before[0].Name != "build" {
		t.Fatalf("expected deploy before=[build], got %+v", g.Root.Before)
	}
}

func TestBuild_DiamondSharesNode(t *testing.T) {
	dir := t.TempDir()
	r := mapResolver{m: map[string]string{
		"a": writeTask(t, dir, "a", "# before:\n#   - b\n#   - c\n"),
		"b": writeTask(t, dir, "b", "# before:\n#   - d\n"),
		"c": writeTask(t, dir, "c", "# before:\n#   - d\n"),
		"d": writeTask(t, dir, "d", ""),
	}}
	g, err := Build("a", r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// b.Before[0] and c.Before[0] must be the SAME *Node (dedup by path).
	if g.Root.Before[0].Before[0] != g.Root.Before[1].Before[0] {
		t.Fatal("expected d to be a shared node pointer")
	}
}

func TestBuild_CycleDetected(t *testing.T) {
	dir := t.TempDir()
	r := mapResolver{m: map[string]string{
		"a": writeTask(t, dir, "a", "# before:\n#   - b\n"),
		"b": writeTask(t, dir, "b", "# before:\n#   - a\n"),
	}}
	_, err := Build("a", r)
	if _, ok := err.(*CycleError); !ok {
		t.Fatalf("expected *CycleError, got %v", err)
	}
}

func TestBuild_UnknownReference(t *testing.T) {
	dir := t.TempDir()
	r := mapResolver{m: map[string]string{
		"a": writeTask(t, dir, "a", "# before:\n#   - ghost\n"),
	}}
	_, err := Build("a", r)
	re, ok := err.(*ResolveError)
	if !ok || re.Kind != ResolveUnknown {
		t.Fatalf("expected ResolveUnknown, got %v", err)
	}
}

func TestBuild_MalformedDeps(t *testing.T) {
	dir := t.TempDir()
	r := mapResolver{m: map[string]string{
		"a": writeTask(t, dir, "a", "# before: build\n"),
	}}
	_, err := Build("a", r)
	if _, ok := err.(*MalformedDepsError); !ok {
		t.Fatalf("expected *MalformedDepsError, got %v", err)
	}
}
