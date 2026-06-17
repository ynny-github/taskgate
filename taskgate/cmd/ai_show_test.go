package cmd

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/ynny-github/taskgate/taskgate/internal/show"
)

// decodeOneJSON asserts buf is a single trailing-\n-terminated JSON object.
func decodeOneJSON(t *testing.T, buf []byte) map[string]any {
	t.Helper()
	if len(buf) == 0 || buf[len(buf)-1] != '\n' {
		t.Fatalf("AI output must end in a single \\n: %q", string(buf))
	}
	var got map[string]any
	if err := json.Unmarshal(buf, &got); err != nil {
		t.Fatalf("not parseable as JSON: %v (%q)", err, string(buf))
	}
	return got
}

func TestAIShow_RootView(t *testing.T) {
	tmp := t.TempDir()
	writeAnnotatedTask(t, filepath.Join(tmp, ".taskgate/ai/analyze"), "Analyze.", "")
	writeAnnotatedTask(t, filepath.Join(tmp, ".taskgate/shared/lint"), "Lint.", "")
	writeAnnotatedTask(t, filepath.Join(tmp, ".taskgate/human/build"), "Build.", "")
	chdirTo(t, tmp)

	var out bytes.Buffer
	root := newRootCmd()
	root.SetArgs([]string{"ai", "show"})
	root.SetOut(&out)
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := decodeOneJSON(t, out.Bytes())
	if got["kind"] != "listing" || got["audience"] != "ai" {
		t.Errorf("envelope = %+v", got)
	}
	rows := got["entries"].([]any)
	names := map[string]bool{}
	for _, row := range rows {
		r := row.(map[string]any)
		names[r["path"].(string)] = true
	}
	if !names[".taskgate/ai/analyze"] || !names[".taskgate/shared/lint"] {
		t.Errorf("missing expected entries: %+v", names)
	}
	if names[".taskgate/human/build"] {
		t.Error("human-only entry leaked into ai listing")
	}
}

func TestAIShow_TaskTarget(t *testing.T) {
	tmp := t.TempDir()
	writeAnnotatedTask(t, filepath.Join(tmp, ".taskgate/ai/analyze"), "Analyze.", "Body!")
	chdirTo(t, tmp)

	var out bytes.Buffer
	root := newRootCmd()
	root.SetArgs([]string{"ai", "show", "analyze"})
	root.SetOut(&out)
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := decodeOneJSON(t, out.Bytes())
	if got["kind"] != "task" {
		t.Errorf("kind = %v", got["kind"])
	}
	if got["body"] != "Body!" {
		t.Errorf("body = %v", got["body"])
	}
}

func TestAIShow_DirectoryTarget(t *testing.T) {
	tmp := t.TempDir()
	writeFile(t, filepath.Join(tmp, ".taskgate/shared/deploy/_index"),
		"# ---\n# summary: Promote.\n# ---\n")
	writeAnnotatedTask(t, filepath.Join(tmp, ".taskgate/shared/deploy/prod"), "Prod.", "")
	chdirTo(t, tmp)

	var out bytes.Buffer
	root := newRootCmd()
	root.SetArgs([]string{"ai", "show", "deploy"})
	root.SetOut(&out)
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := decodeOneJSON(t, out.Bytes())
	if got["kind"] != "directory" {
		t.Errorf("kind = %v", got["kind"])
	}
	if got["summary"] != "Promote." {
		t.Errorf("summary = %v", got["summary"])
	}
	entries := got["entries"].([]any)
	if len(entries) != 1 {
		t.Fatalf("expected 1 child, got %d", len(entries))
	}
}

func TestAIShow_Collision_Exit4_StdoutEnvelope(t *testing.T) {
	tmp := t.TempDir()
	writeAnnotatedTask(t, filepath.Join(tmp, ".taskgate/ai/build"), "a.", "")
	writeAnnotatedTask(t, filepath.Join(tmp, ".taskgate/shared/build"), "s.", "")
	chdirTo(t, tmp)

	var out, errBuf bytes.Buffer
	root := newRootCmd()
	root.SetArgs([]string{"ai", "show"})
	root.SetOut(&out)
	root.SetErr(&errBuf)
	err := root.Execute()
	wantExit(t, err, show.ExitCollision)
	got := decodeOneJSON(t, out.Bytes())
	if got["error"] != "collision" {
		t.Errorf("envelope = %+v", got)
	}
	if errBuf.Len() != 0 {
		t.Errorf("stderr must stay empty for AI form, got %q", errBuf.String())
	}
}

func TestAIShow_InvalidInput_Exit2_StdoutEnvelope(t *testing.T) {
	tmp := t.TempDir()
	writeAnnotatedTask(t, filepath.Join(tmp, ".taskgate/ai/analyze"), "a.", "")
	chdirTo(t, tmp)

	var out bytes.Buffer
	root := newRootCmd()
	root.SetArgs([]string{"ai", "show", ".taskgate/ai/analyze"})
	root.SetOut(&out)
	err := root.Execute()
	wantExit(t, err, show.ExitInvalidInput)
	got := decodeOneJSON(t, out.Bytes())
	if got["error"] != "invalid_input" || got["reason"] != "filesystem_path" {
		t.Errorf("envelope = %+v", got)
	}
}

func TestAIShow_NotFound_Exit3_StdoutEnvelope(t *testing.T) {
	tmp := t.TempDir()
	writeAnnotatedTask(t, filepath.Join(tmp, ".taskgate/ai/analyze"), "a.", "")
	chdirTo(t, tmp)

	var out bytes.Buffer
	root := newRootCmd()
	root.SetArgs([]string{"ai", "show", "missing"})
	root.SetOut(&out)
	err := root.Execute()
	wantExit(t, err, show.ExitNotFound)
	got := decodeOneJSON(t, out.Bytes())
	if got["error"] != "not_found" {
		t.Errorf("envelope = %+v", got)
	}
}

