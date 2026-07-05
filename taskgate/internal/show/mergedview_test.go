package show

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ynny-github/taskgate/taskgate/internal/annotation"
)

// writeFixture writes a file at <ws>/<rel> with the given contents. Creates
// parent dirs as needed. Used to build merged-view fixtures.
func writeFixture(t *testing.T, ws, rel, contents string) {
	t.Helper()
	abs := filepath.Join(ws, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}
}

func writeTaskFixture(t *testing.T, ws, rel, summary, body string) {
	t.Helper()
	parts := []string{"#!/bin/sh", "# ---"}
	if summary != "" {
		parts = append(parts, "# summary: "+summary)
	}
	if body != "" {
		parts = append(parts, "# body: |")
		for _, line := range strings.Split(body, "\n") {
			parts = append(parts, "#   "+line)
		}
	}
	parts = append(parts, "# ---", "echo hi", "")
	writeFixture(t, ws, rel, strings.Join(parts, "\n"))
	if err := os.Chmod(filepath.Join(ws, rel), 0o755); err != nil {
		t.Fatal(err)
	}
}

func mkdir(t *testing.T, ws, rel string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(ws, rel), 0755); err != nil {
		t.Fatal(err)
	}
}

// resolveRoot is a convenience that creates the .taskgate root dir if
// missing then calls ResolveRoot against it.
func resolveRoot(t *testing.T, tmp string, audience Audience) ([]Entry, *CollisionReport) {
	t.Helper()
	ws := filepath.Join(tmp, ".taskgate")
	if err := os.MkdirAll(ws, 0755); err != nil {
		t.Fatal(err)
	}
	entries, col, err := ResolveRoot(audience, ws)
	if err != nil {
		t.Fatalf("ResolveRoot: %v", err)
	}
	return entries, col
}

func TestResolveRoot_HumanMergesSharedAndHuman(t *testing.T) {
	tmp := t.TempDir()
	writeTaskFixture(t, tmp, ".taskgate/human/build", "Build the project.", "")
	writeTaskFixture(t, tmp, ".taskgate/shared/lint", "Lint the codebase.", "")
	writeFixture(t, tmp, ".taskgate/shared/test", "#!/bin/sh\necho hi\n") // no annotation
	if err := os.Chmod(filepath.Join(tmp, ".taskgate/shared/test"), 0o755); err != nil {
		t.Fatal(err)
	}

	entries, col := resolveRoot(t, tmp, AudienceHuman)
	if col != nil {
		t.Fatalf("unexpected collision: %+v", col)
	}
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3: %+v", len(entries), entries)
	}
	byName := map[string]Entry{}
	for _, e := range entries {
		byName[e.Name] = e
	}
	wantPaths := map[string]string{
		"build": ".taskgate/human/build",
		"lint":  ".taskgate/shared/lint",
		"test":  ".taskgate/shared/test",
	}
	for name, want := range wantPaths {
		got, ok := byName[name]
		if !ok {
			t.Errorf("missing entry %q", name)
			continue
		}
		if got.Path != want {
			t.Errorf("entry %q path = %q, want %q", name, got.Path, want)
		}
	}
	if byName["build"].Annotation.Summary != "Build the project." {
		t.Errorf("build summary = %q", byName["build"].Annotation.Summary)
	}
	if byName["test"].Annotation.Summary != "" {
		t.Errorf("test should have empty summary, got %q", byName["test"].Annotation.Summary)
	}
}

func TestResolveRoot_AISkipsHuman(t *testing.T) {
	tmp := t.TempDir()
	writeTaskFixture(t, tmp, ".taskgate/human/build", "Build.", "")
	writeTaskFixture(t, tmp, ".taskgate/ai/analyze", "Analyze.", "")
	writeTaskFixture(t, tmp, ".taskgate/shared/lint", "Lint.", "")

	entries, col := resolveRoot(t, tmp, AudienceAI)
	if col != nil {
		t.Fatalf("unexpected collision: %+v", col)
	}
	names := map[string]bool{}
	for _, e := range entries {
		names[e.Name] = true
	}
	if names["build"] {
		t.Errorf("AI view leaked human-only entry 'build'")
	}
	if !names["analyze"] || !names["lint"] {
		t.Errorf("AI view missing expected entries: %+v", entries)
	}
}

func TestResolveRoot_SortOrder(t *testing.T) {
	tmp := t.TempDir()
	mkdir(t, tmp, ".taskgate/human/deploy")
	writeTaskFixture(t, tmp, ".taskgate/human/build", "", "")
	writeTaskFixture(t, tmp, ".taskgate/shared/lint", "", "")
	writeTaskFixture(t, tmp, ".taskgate/human/Alpha", "", "") // case-sensitive: uppercase sorts first

	entries, _ := resolveRoot(t, tmp, AudienceHuman)
	if len(entries) != 4 {
		t.Fatalf("want 4 entries, got %d: %+v", len(entries), entries)
	}
	// Directories first ("deploy"), then tasks: "Alpha", "build", "lint".
	wantOrder := []string{"deploy", "Alpha", "build", "lint"}
	for i, name := range wantOrder {
		if entries[i].Name != name {
			t.Errorf("position %d = %q, want %q (full order: %v)", i, entries[i].Name, name, entryNames(entries))
		}
	}
}

func entryNames(entries []Entry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Name
	}
	return out
}

func TestResolveRoot_CollisionFileVsFile(t *testing.T) {
	tmp := t.TempDir()
	writeTaskFixture(t, tmp, ".taskgate/human/build", "human.", "")
	writeTaskFixture(t, tmp, ".taskgate/shared/build", "shared.", "")

	entries, col := resolveRoot(t, tmp, AudienceHuman)
	if col == nil {
		t.Fatalf("expected collision, got entries: %+v", entries)
	}
	if col.Name != "build" {
		t.Errorf("collision name = %q, want build", col.Name)
	}
	if len(col.Paths) != 2 {
		t.Errorf("want 2 paths, got %d: %v", len(col.Paths), col.Paths)
	}
}

func TestResolveRoot_CollisionDirVsFile(t *testing.T) {
	tmp := t.TempDir()
	mkdir(t, tmp, ".taskgate/human/build")
	writeTaskFixture(t, tmp, ".taskgate/shared/build", "shared.", "")

	_, col := resolveRoot(t, tmp, AudienceHuman)
	if col == nil {
		t.Fatal("expected collision on dir-vs-file")
	}
}

func TestResolveRoot_NoCollisionAcrossNamespaces(t *testing.T) {
	tmp := t.TempDir()
	writeTaskFixture(t, tmp, ".taskgate/human/build", "h.", "")
	writeTaskFixture(t, tmp, ".taskgate/shared/lint", "s.", "")

	_, col := resolveRoot(t, tmp, AudienceHuman)
	if col != nil {
		t.Fatalf("unexpected collision on disjoint names: %+v", col)
	}
}

func TestValidateName_RejectsAbsolutePath(t *testing.T) {
	if r := ValidateName("/abs/path"); r == nil || r.Reason != "absolute_path" {
		t.Errorf("got %+v, want absolute_path", r)
	}
}

func TestValidateName_RejectsCwdRelative(t *testing.T) {
	if r := ValidateName("./build"); r == nil || r.Reason != "parent_escape" {
		t.Errorf("./build: got %+v, want parent_escape", r)
	}
	if r := ValidateName("../build"); r == nil || r.Reason != "parent_escape" {
		t.Errorf("../build: got %+v, want parent_escape", r)
	}
}

func TestValidateName_RejectsTaskgatePrefix(t *testing.T) {
	if r := ValidateName(".taskgate/human/build"); r == nil || r.Reason != "filesystem_path" {
		t.Errorf("got %+v, want filesystem_path", r)
	}
}

func TestValidateName_RejectsEmpty(t *testing.T) {
	if r := ValidateName(""); r == nil || r.Reason != "empty" {
		t.Errorf("got %+v, want empty", r)
	}
}

func TestValidateName_AcceptsBareName(t *testing.T) {
	if r := ValidateName("build"); r != nil {
		t.Errorf("got %+v, want nil", r)
	}
}

func TestValidateName_AcceptsNestedName(t *testing.T) {
	if r := ValidateName("deploy/prod"); r != nil {
		t.Errorf("got %+v, want nil", r)
	}
}

func TestResolveName_Task_ReturnsTaskTarget(t *testing.T) {
	tmp := t.TempDir()
	writeTaskFixture(t, tmp, ".taskgate/human/build", "Build.", "Multi\nline")
	ws := filepath.Join(tmp, ".taskgate")

	target, col, nf, err := ResolveName(AudienceHuman, ws, "build")
	if err != nil {
		t.Fatal(err)
	}
	if col != nil || nf != nil {
		t.Fatalf("unexpected col=%+v nf=%+v", col, nf)
	}
	if target.Kind != EntryKindTask {
		t.Errorf("kind = %v, want task", target.Kind)
	}
	if target.Entry.Path != ".taskgate/human/build" {
		t.Errorf("path = %q", target.Entry.Path)
	}
}

func TestResolveName_Task_NestedPath(t *testing.T) {
	tmp := t.TempDir()
	writeTaskFixture(t, tmp, ".taskgate/human/deploy/prod", "Prod.", "")
	ws := filepath.Join(tmp, ".taskgate")

	target, _, nf, err := ResolveName(AudienceHuman, ws, "deploy/prod")
	if err != nil {
		t.Fatal(err)
	}
	if nf != nil {
		t.Fatalf("not_found unexpectedly: %+v", nf)
	}
	if target.Entry.Path != ".taskgate/human/deploy/prod" {
		t.Errorf("path = %q", target.Entry.Path)
	}
}

func TestResolveName_NotFound(t *testing.T) {
	tmp := t.TempDir()
	mkdir(t, tmp, ".taskgate")
	ws := filepath.Join(tmp, ".taskgate")

	_, _, nf, err := ResolveName(AudienceHuman, ws, "nope")
	if err != nil {
		t.Fatal(err)
	}
	if nf == nil {
		t.Fatal("expected not_found, got nil")
	}
	if nf.Name != "nope" {
		t.Errorf("name = %q", nf.Name)
	}
}

func TestResolveName_Task_CollisionInBuckets(t *testing.T) {
	tmp := t.TempDir()
	writeTaskFixture(t, tmp, ".taskgate/human/build", "h.", "")
	writeTaskFixture(t, tmp, ".taskgate/shared/build", "s.", "")
	ws := filepath.Join(tmp, ".taskgate")

	_, col, _, err := ResolveName(AudienceHuman, ws, "build")
	if err != nil {
		t.Fatal(err)
	}
	if col == nil {
		t.Fatal("expected collision")
	}
}

func TestResolveName_Directory_HasNoAnnotation(t *testing.T) {
	tmp := t.TempDir()
	// A non-executable _index is now just a non-executable file: filtered out.
	writeFixture(t, tmp, ".taskgate/human/deploy/_index", "# ---\n# summary: Promote.\n# ---\n")
	writeTaskFixture(t, tmp, ".taskgate/human/deploy/prod", "Prod.", "")
	ws := filepath.Join(tmp, ".taskgate")

	target, _, nf, err := ResolveName(AudienceHuman, ws, "deploy")
	if err != nil {
		t.Fatal(err)
	}
	if nf != nil {
		t.Fatalf("not found: %+v", nf)
	}
	if target.Kind != EntryKindDirectory {
		t.Fatalf("kind = %v, want directory", target.Kind)
	}
	if target.Entry.Annotation != (annotation.AnnotationBlock{}) {
		t.Errorf("directory must carry no annotation, got %+v", target.Entry.Annotation)
	}
	for _, c := range target.Children {
		if c.Name == "_index" {
			t.Error("non-executable _index must be filtered from children")
		}
	}
}

func TestResolveName_Directory_WithoutIndex(t *testing.T) {
	tmp := t.TempDir()
	writeTaskFixture(t, tmp, ".taskgate/human/deploy/prod", "Prod.", "")
	ws := filepath.Join(tmp, ".taskgate")

	target, _, _, err := ResolveName(AudienceHuman, ws, "deploy")
	if err != nil {
		t.Fatal(err)
	}
	if target == nil || target.Kind != EntryKindDirectory {
		t.Fatalf("want directory, got %+v", target)
	}
	if target.Entry.Annotation != (annotation.AnnotationBlock{}) {
		t.Errorf("annotation should be zero, got %+v", target.Entry.Annotation)
	}
}

func TestResolveName_Directory_NestedPath(t *testing.T) {
	tmp := t.TempDir()
	writeTaskFixture(t, tmp, ".taskgate/human/release/canary/promote", "Promote.", "")
	ws := filepath.Join(tmp, ".taskgate")

	target, _, _, err := ResolveName(AudienceHuman, ws, "release/canary")
	if err != nil {
		t.Fatal(err)
	}
	if target.Kind != EntryKindDirectory {
		t.Errorf("kind = %v", target.Kind)
	}
	if target.Entry.Path != ".taskgate/human/release/canary" {
		t.Errorf("path = %q", target.Entry.Path)
	}
}

func TestResolveName_Directory_ChildrenSorted(t *testing.T) {
	tmp := t.TempDir()
	writeTaskFixture(t, tmp, ".taskgate/human/deploy/prod", "", "")
	writeTaskFixture(t, tmp, ".taskgate/human/deploy/canary", "", "")
	mkdir(t, tmp, ".taskgate/human/deploy/sub")
	ws := filepath.Join(tmp, ".taskgate")

	target, _, _, err := ResolveName(AudienceHuman, ws, "deploy")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"sub", "canary", "prod"}
	if len(target.Children) != len(want) {
		t.Fatalf("got %d children, want %d", len(target.Children), len(want))
	}
	for i, n := range want {
		if target.Children[i].Name != n {
			t.Errorf("position %d = %q, want %q (%v)", i, target.Children[i].Name, n, entryNames(target.Children))
		}
	}
}

func TestResolveName_Directory_ExecutableIndexIsOrdinaryChild(t *testing.T) {
	tmp := t.TempDir()
	idx := filepath.Join(tmp, ".taskgate/human/deploy/_index")
	if err := os.MkdirAll(filepath.Dir(idx), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(idx, []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTaskFixture(t, tmp, ".taskgate/human/deploy/prod", "", "")
	ws := filepath.Join(tmp, ".taskgate")

	target, _, _, err := ResolveName(AudienceHuman, ws, "deploy")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, c := range target.Children {
		names = append(names, c.Name)
	}
	found := false
	for _, n := range names {
		if n == "_index" {
			found = true
		}
	}
	if !found {
		t.Errorf("executable _index must appear as an ordinary child, got %v", names)
	}
}

func TestResolveName_Directory_NoRecursion(t *testing.T) {
	tmp := t.TempDir()
	writeTaskFixture(t, tmp, ".taskgate/human/deploy/sub/grand", "g.", "")
	ws := filepath.Join(tmp, ".taskgate")

	target, _, _, err := ResolveName(AudienceHuman, ws, "deploy")
	if err != nil {
		t.Fatal(err)
	}
	names := entryNames(target.Children)
	if len(names) != 1 || names[0] != "sub" {
		t.Errorf("expected only direct child 'sub', got %v", names)
	}
}

func TestResolveName_Directory_CollisionInChildren(t *testing.T) {
	tmp := t.TempDir()
	writeTaskFixture(t, tmp, ".taskgate/human/deploy/prod", "h.", "")
	writeTaskFixture(t, tmp, ".taskgate/shared/deploy/prod", "s.", "")
	ws := filepath.Join(tmp, ".taskgate")

	// Writing children into both buckets makes both `<bucket>/deploy/`
	// dirs exist, which is itself a parent-level collision per FR-013.
	// Either path satisfies "do not produce partial output for the
	// conflicting region" — assert collision fires somewhere.
	_, col, _, err := ResolveName(AudienceHuman, ws, "deploy")
	if err != nil {
		t.Fatal(err)
	}
	if col == nil {
		t.Fatal("expected collision report")
	}
}

func TestResolveRoot_UnreadableExecutableFile(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses file permissions; this test only runs for unprivileged users")
	}
	tmp := t.TempDir()
	writeTaskFixture(t, tmp, ".taskgate/human/readable", "Readable.", "")
	unreadable := filepath.Join(tmp, ".taskgate/human/unreadable")
	if err := os.MkdirAll(filepath.Dir(unreadable), 0o755); err != nil {
		t.Fatal(err)
	}
	// Executable so it survives the exec filter, but unreadable so the
	// annotation read fails and surfaces a note.
	if err := os.WriteFile(unreadable, []byte("#!/bin/sh\necho hi\n"), 0o111); err != nil {
		t.Fatal(err)
	}

	entries, col := resolveRoot(t, tmp, AudienceHuman)
	if col != nil {
		t.Fatalf("unexpected collision: %+v", col)
	}
	names := map[string]Entry{}
	for _, e := range entries {
		names[e.Name] = e
	}
	if _, ok := names["unreadable"]; !ok {
		t.Fatal("executable-but-unreadable entry should still surface")
	}
	if names["unreadable"].Note == "" {
		t.Errorf("Note should describe the read failure")
	}
}

func TestResolveRoot_SymlinkEscapesWorkspace(t *testing.T) {
	tmp := t.TempDir()
	external := filepath.Join(tmp, "external-target.sh")
	if err := os.WriteFile(external, []byte("#!/bin/sh\necho external\n"), 0755); err != nil {
		t.Fatal(err)
	}
	mkdir(t, tmp, ".taskgate/human")
	link := filepath.Join(tmp, ".taskgate/human/escapee")
	if err := os.Symlink(external, link); err != nil {
		t.Fatal(err)
	}

	entries, col := resolveRoot(t, tmp, AudienceHuman)
	if col != nil {
		t.Fatalf("unexpected collision: %+v", col)
	}
	var got Entry
	for _, e := range entries {
		if e.Name == "escapee" {
			got = e
		}
	}
	if got.Name == "" {
		t.Fatal("symlink entry missing from listing")
	}
	if got.Annotation != (annotation.AnnotationBlock{}) {
		t.Error("annotation must NOT be parsed for an escaping symlink")
	}
	if got.Note == "" {
		t.Error("Note should describe the symlink escape")
	}
}

func TestResolveRoot_NonExecutableFileHidden(t *testing.T) {
	tmp := t.TempDir()
	writeTaskFixture(t, tmp, ".taskgate/human/build", "Build.", "")
	writeFixture(t, tmp, ".taskgate/human/notes.txt", "just notes\n") // 0644, non-exec
	// stray non-executable _index: also hidden, now purely because it is non-exec.
	writeFixture(t, tmp, ".taskgate/human/_index", "---\nsummary: bucket\n---\n")

	entries, col := resolveRoot(t, tmp, AudienceHuman)
	if col != nil {
		t.Fatalf("unexpected collision: %+v", col)
	}
	for _, e := range entries {
		if e.Name == "notes.txt" || e.Name == "_index" {
			t.Errorf("non-executable file %q must be hidden", e.Name)
		}
	}
	if len(entries) != 1 || entries[0].Name != "build" {
		t.Errorf("want only executable 'build', got %v", entryNames(entries))
	}
}
