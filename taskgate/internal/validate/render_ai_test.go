package validate

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/ynny-github/taskgate/taskgate/internal/show"
)

func TestRenderAI_EmptyIsOKTrue(t *testing.T) {
	var stdout bytes.Buffer
	code, err := renderAI(&stdout, nil)
	if err != nil {
		t.Fatal(err)
	}
	if code != show.ExitSuccess {
		t.Errorf("code = %d, want %d", code, show.ExitSuccess)
	}
	var env struct {
		Kind     string        `json:"kind"`
		OK       bool          `json:"ok"`
		Findings []json.RawMessage `json:"findings"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("invalid JSON %q: %v", stdout.String(), err)
	}
	if env.Kind != "validation" || !env.OK {
		t.Errorf("env = %+v", env)
	}
	if env.Findings == nil {
		t.Errorf("findings must be [] not null")
	}
}

func TestRenderAI_FindingsSetOKFalse(t *testing.T) {
	var stdout bytes.Buffer
	code, err := renderAI(&stdout, []Finding{
		{Rule: RuleExecBit, Path: ".taskgate/human/deploy", Message: "task file is not executable"},
		{Rule: RuleCollision, Name: "build", Paths: []string{".taskgate/shared/build", ".taskgate/human/build"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if code != show.ExitGeneric {
		t.Errorf("code = %d, want %d", code, show.ExitGeneric)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"ok":false`)) {
		t.Errorf("expected ok:false, got %q", stdout.String())
	}
	// Collision finding must omit path/message; file finding must omit name/paths.
	if bytes.Contains(stdout.Bytes(), []byte(`"path":""`)) || bytes.Contains(stdout.Bytes(), []byte(`"name":""`)) {
		t.Errorf("empty fields must be omitted, got %q", stdout.String())
	}
	if got := stdout.Bytes()[stdout.Len()-1]; got != '\n' {
		t.Errorf("expected trailing newline")
	}
}

func TestWriteAIError(t *testing.T) {
	var buf bytes.Buffer
	if err := writeAIError(&buf, "not_found", "task \"x\" not found"); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"kind":"error"`)) || !bytes.Contains(buf.Bytes(), []byte(`"error":"not_found"`)) {
		t.Errorf("output = %q", buf.String())
	}
}
