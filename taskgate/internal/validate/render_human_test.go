package validate

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ynny-github/taskgate/taskgate/internal/show"
)

func TestRenderHuman_EmptyIsSilentSuccess(t *testing.T) {
	var stderr bytes.Buffer
	code, err := renderHuman(&stderr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if code != show.ExitSuccess {
		t.Errorf("code = %d, want %d", code, show.ExitSuccess)
	}
	if stderr.Len() != 0 {
		t.Errorf("expected no output, got %q", stderr.String())
	}
}

func TestRenderHuman_FileFindingLine(t *testing.T) {
	var stderr bytes.Buffer
	code, err := renderHuman(&stderr, []Finding{
		{Rule: RuleExecBit, Path: ".taskgate/human/deploy", Message: "task file is not executable"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if code != show.ExitGeneric {
		t.Errorf("code = %d, want %d", code, show.ExitGeneric)
	}
	want := ".taskgate/human/deploy: exec-bit: task file is not executable\n"
	if stderr.String() != want {
		t.Errorf("output = %q, want %q", stderr.String(), want)
	}
}

func TestRenderHuman_CollisionLine(t *testing.T) {
	var stderr bytes.Buffer
	_, err := renderHuman(&stderr, []Finding{
		{Rule: RuleCollision, Name: "build", Paths: []string{".taskgate/shared/build", ".taskgate/human/build"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "collision: build: .taskgate/shared/build, .taskgate/human/build\n"
	if stderr.String() != want {
		t.Errorf("output = %q, want %q", stderr.String(), want)
	}
}

func TestRenderNotFound_HumanWritesStderr(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := renderNotFound(show.AudienceHuman, show.NotFoundReport{Name: "nope"}, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if code != show.ExitNotFound {
		t.Errorf("code = %d, want %d", code, show.ExitNotFound)
	}
	if !strings.Contains(stderr.String(), "nope") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

func TestRenderNotFound_AIWritesEnvelopeWithNameAndSearched(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := renderNotFound(show.AudienceAI, show.NotFoundReport{
		Name:     "nope",
		Searched: []string{".taskgate/human"},
	}, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if code != show.ExitNotFound {
		t.Errorf("code = %d, want %d", code, show.ExitNotFound)
	}
	if stderr.Len() != 0 {
		t.Errorf("expected no stderr output, got %q", stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{`"kind":"error"`, `"error":"not_found"`, `"name":"nope"`, `.taskgate/human`} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q; got %q", want, out)
		}
	}
}
