package show

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ynny-github/taskgate/taskgate/internal/annotation"
)

// decodeOneJSON asserts that buf contains exactly one JSON document
// followed by a single trailing "\n" and returns it as a generic map.
func decodeOneJSON(t *testing.T, buf []byte) map[string]any {
	t.Helper()
	if len(buf) == 0 || buf[len(buf)-1] != '\n' {
		t.Fatalf("AI output must end in a single \\n: %q", string(buf))
	}
	var got map[string]any
	dec := json.NewDecoder(bytes.NewReader(buf))
	if err := dec.Decode(&got); err != nil {
		t.Fatalf("not parseable as JSON: %v (%q)", err, string(buf))
	}
	// Verify there's no second document.
	if dec.More() {
		t.Fatalf("expected a single JSON document, got extra bytes: %q", string(buf))
	}
	return got
}

func TestRenderHumanTask_WithBody(t *testing.T) {
	e := Entry{
		Path: ".taskgate/human/build",
		Kind: EntryKindTask,
		Annotation: annotation.AnnotationBlock{
			Summary: "Build.",
			Body:    "Multi\nline body.",
		},
	}
	var buf bytes.Buffer
	if err := RenderHumanTask(&buf, e); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, ".taskgate/human/build") {
		t.Errorf("missing path: %q", got)
	}
	if !strings.Contains(got, "  Build.") {
		t.Errorf("missing indented summary: %q", got)
	}
	if !strings.Contains(got, "Multi\nline body.") {
		t.Errorf("missing body: %q", got)
	}
}

func TestRenderHumanTask_SummaryOnly(t *testing.T) {
	e := Entry{
		Path: ".taskgate/shared/lint",
		Kind: EntryKindTask,
		Annotation: annotation.AnnotationBlock{
			Summary: "Lint.",
		},
	}
	var buf bytes.Buffer
	if err := RenderHumanTask(&buf, e); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	// Body section must be absent — no trailing blank-line + content.
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	for _, line := range lines {
		if line == "Multi" || strings.HasPrefix(line, "body") {
			t.Errorf("body sneaked in: %q", got)
		}
	}
}

func TestRenderHumanDirectory_PathThenChildren(t *testing.T) {
	target := ResolvedTarget{
		Kind:  EntryKindDirectory,
		Entry: Entry{Path: ".taskgate/human/deploy", Kind: EntryKindDirectory},
		Children: []Entry{
			{Name: "sub", Path: ".taskgate/human/deploy/sub", Kind: EntryKindDirectory},
			{Name: "canary", Path: ".taskgate/human/deploy/canary", Kind: EntryKindTask,
				Annotation: annotation.AnnotationBlock{Summary: "Canary."}},
			{Name: "prod", Path: ".taskgate/human/deploy/prod", Kind: EntryKindTask},
		},
	}
	var buf bytes.Buffer
	if err := RenderHumanDirectory(&buf, target); err != nil {
		t.Fatal(err)
	}
	want := ".taskgate/human/deploy\n\n  sub/\n  canary\tCanary.\n  prod\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

func TestRenderHumanTree_IndentsByDepth(t *testing.T) {
	entries := []Entry{
		{Name: "deploy", Kind: EntryKindDirectory, Depth: 0},
		{Name: "prod", Kind: EntryKindTask, Depth: 1,
			Annotation: annotation.AnnotationBlock{Summary: "Prod."}},
		{Name: "build", Kind: EntryKindTask, Depth: 0,
			Annotation: annotation.AnnotationBlock{Summary: "Build."}},
		{Name: "bare", Kind: EntryKindTask, Depth: 0},
	}
	var buf bytes.Buffer
	if err := RenderHumanTree(&buf, entries); err != nil {
		t.Fatal(err)
	}
	want := "deploy/\n  prod\tProd.\nbuild\tBuild.\nbare\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

func TestRenderAI_Listing(t *testing.T) {
	entries := []Entry{
		{Name: "deploy", Path: ".taskgate/shared/deploy", Kind: EntryKindDirectory,
			Annotation: annotation.AnnotationBlock{Summary: "Promote."}},
		{Name: "lint", Path: ".taskgate/shared/lint", Kind: EntryKindTask},
	}
	var buf bytes.Buffer
	if err := renderAIRootListing(&buf, entries); err != nil {
		t.Fatal(err)
	}
	got := decodeOneJSON(t, buf.Bytes())
	if got["kind"] != "listing" {
		t.Errorf("kind = %v", got["kind"])
	}
	if got["audience"] != "ai" {
		t.Errorf("audience = %v", got["audience"])
	}
	rows, ok := got["entries"].([]any)
	if !ok || len(rows) != 2 {
		t.Fatalf("entries = %v", got["entries"])
	}
	r0 := rows[0].(map[string]any)
	if r0["kind"] != "directory" || r0["summary"] != "Promote." {
		t.Errorf("row0 = %+v", r0)
	}
	r1 := rows[1].(map[string]any)
	if r1["kind"] != "task" {
		t.Errorf("row1 kind = %v", r1["kind"])
	}
	if _, present := r1["summary"]; !present {
		t.Error("summary must be present even when null")
	}
	if r1["summary"] != nil {
		t.Errorf("row1 summary = %v, want null", r1["summary"])
	}
}

func TestRenderAI_Task(t *testing.T) {
	target := ResolvedTarget{
		Kind: EntryKindTask,
		Entry: Entry{
			Path: ".taskgate/shared/lint",
			Kind: EntryKindTask,
			Annotation: annotation.AnnotationBlock{
				Summary: "Lint.",
				Body:    "Body.",
			},
		},
	}
	var buf bytes.Buffer
	if err := renderAITarget(&buf, target); err != nil {
		t.Fatal(err)
	}
	got := decodeOneJSON(t, buf.Bytes())
	if got["kind"] != "task" {
		t.Errorf("kind = %v", got["kind"])
	}
	if got["body"] != "Body." {
		t.Errorf("body = %v", got["body"])
	}
	if got["audience"] != "ai" {
		t.Errorf("audience = %v", got["audience"])
	}
}

func TestRenderAI_Task_NoBodyOmitted(t *testing.T) {
	target := ResolvedTarget{
		Kind: EntryKindTask,
		Entry: Entry{
			Path:       ".taskgate/shared/lint",
			Kind:       EntryKindTask,
			Annotation: annotation.AnnotationBlock{Summary: "Lint."},
		},
	}
	var buf bytes.Buffer
	if err := renderAITarget(&buf, target); err != nil {
		t.Fatal(err)
	}
	got := decodeOneJSON(t, buf.Bytes())
	if _, present := got["body"]; present {
		t.Errorf("body should be omitted, got envelope: %v", got)
	}
}

func TestRenderAI_Task_NullSummary(t *testing.T) {
	target := ResolvedTarget{
		Kind: EntryKindTask,
		Entry: Entry{
			Path: ".taskgate/shared/test",
			Kind: EntryKindTask,
		},
	}
	var buf bytes.Buffer
	if err := renderAITarget(&buf, target); err != nil {
		t.Fatal(err)
	}
	got := decodeOneJSON(t, buf.Bytes())
	if _, present := got["summary"]; !present {
		t.Error("summary must be present (as null)")
	}
	if got["summary"] != nil {
		t.Errorf("summary = %v, want nil", got["summary"])
	}
}

func TestRenderAI_Directory(t *testing.T) {
	target := ResolvedTarget{
		Kind:  EntryKindDirectory,
		Entry: Entry{Path: ".taskgate/shared/deploy", Kind: EntryKindDirectory},
		Children: []Entry{
			{Name: "prod", Path: ".taskgate/shared/deploy/prod", Kind: EntryKindTask,
				Annotation: annotation.AnnotationBlock{Summary: "Prod."}},
		},
	}
	var buf bytes.Buffer
	if err := renderAITarget(&buf, target); err != nil {
		t.Fatal(err)
	}
	got := decodeOneJSON(t, buf.Bytes())
	if got["kind"] != "directory" {
		t.Errorf("kind = %v", got["kind"])
	}
	if _, present := got["summary"]; present {
		t.Errorf("directory envelope must not carry summary: %v", got)
	}
	if _, present := got["body"]; present {
		t.Errorf("directory envelope must not carry body: %v", got)
	}
	rows, ok := got["entries"].([]any)
	if !ok || len(rows) != 1 {
		t.Fatalf("entries = %v", got["entries"])
	}
}

func TestRenderAI_Error_Collision(t *testing.T) {
	env := errorEnvelope{
		Kind: "error", Err: "collision", Message: "x",
		Name: "build", Paths: []string{"a", "b"},
	}
	var buf bytes.Buffer
	if err := renderAIError(&buf, env); err != nil {
		t.Fatal(err)
	}
	got := decodeOneJSON(t, buf.Bytes())
	if got["error"] != "collision" || got["name"] != "build" {
		t.Errorf("got %+v", got)
	}
}

func TestRenderAI_Error_NotFound(t *testing.T) {
	env := errorEnvelope{
		Kind: "error", Err: "not_found", Message: "x",
		Name: "foo", Searched: []string{"a"},
	}
	var buf bytes.Buffer
	if err := renderAIError(&buf, env); err != nil {
		t.Fatal(err)
	}
	got := decodeOneJSON(t, buf.Bytes())
	if got["error"] != "not_found" {
		t.Errorf("got %+v", got)
	}
}

func TestRenderAI_Error_InvalidInput(t *testing.T) {
	env := errorEnvelope{
		Kind: "error", Err: "invalid_input", Message: "x",
		Input: "/abs", Reason: "absolute_path",
	}
	var buf bytes.Buffer
	if err := renderAIError(&buf, env); err != nil {
		t.Fatal(err)
	}
	got := decodeOneJSON(t, buf.Bytes())
	if got["error"] != "invalid_input" || got["reason"] != "absolute_path" {
		t.Errorf("got %+v", got)
	}
}

func TestRenderAI_Error_WorkspaceMissing(t *testing.T) {
	env := errorEnvelope{
		Kind: "error", Err: "workspace_missing", Message: "x",
	}
	var buf bytes.Buffer
	if err := renderAIError(&buf, env); err != nil {
		t.Fatal(err)
	}
	got := decodeOneJSON(t, buf.Bytes())
	if got["error"] != "workspace_missing" {
		t.Errorf("got %+v", got)
	}
}

func TestRenderAI_Error_IO(t *testing.T) {
	env := errorEnvelope{
		Kind: "error", Err: "io", Message: "x", Path: ".taskgate/shared/build",
	}
	var buf bytes.Buffer
	if err := renderAIError(&buf, env); err != nil {
		t.Fatal(err)
	}
	got := decodeOneJSON(t, buf.Bytes())
	if got["error"] != "io" || got["path"] != ".taskgate/shared/build" {
		t.Errorf("got %+v", got)
	}
}

func TestRenderHumanDirectory_NoChildren(t *testing.T) {
	target := ResolvedTarget{
		Kind:  EntryKindDirectory,
		Entry: Entry{Path: ".taskgate/human/empty", Kind: EntryKindDirectory},
	}
	var buf bytes.Buffer
	if err := RenderHumanDirectory(&buf, target); err != nil {
		t.Fatal(err)
	}
	if buf.String() != ".taskgate/human/empty\n" {
		t.Errorf("got %q, want path-only line", buf.String())
	}
}

func TestRenderHumanTask_PathOnly(t *testing.T) {
	e := Entry{
		Path: ".taskgate/shared/test",
		Kind: EntryKindTask,
	}
	var buf bytes.Buffer
	if err := RenderHumanTask(&buf, e); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(buf.String()) != ".taskgate/shared/test" {
		t.Errorf("got %q, want just the path", buf.String())
	}
}

func TestRunName(t *testing.T) {
	cases := map[string]string{
		".taskgate/human/build":        "build",
		".taskgate/shared/deploy/prod":  "deploy/prod",
		".taskgate/shared/deploy":       "deploy",
		".taskgate/ai/deep/nested/task": "deep/nested/task",
	}
	for path, want := range cases {
		if got := runName(path); got != want {
			t.Errorf("runName(%q) = %q, want %q", path, got, want)
		}
	}
}
