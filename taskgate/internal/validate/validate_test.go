package validate

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ynny-github/taskgate/taskgate/internal/show"
)

// setupWS builds a .taskgate workspace in a temp dir and chdirs into its
// parent so Run's os.Getwd()+WorkspaceDir finds it. Returns the workspace dir.
func setupWS(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	ws := filepath.Join(root, ".taskgate")
	for _, b := range []string{"human", "ai", "shared"} {
		if err := os.MkdirAll(filepath.Join(ws, b), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
	return ws
}

func TestRun_CleanTreeSucceedsSilently(t *testing.T) {
	ws := setupWS(t)
	mkTask(t, ws, "shared", "build", "#!/bin/sh\n# ---\n# summary: Build.\n# ---\n")

	var stdout, stderr bytes.Buffer
	code, err := Run(show.AudienceHuman, nil, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if code != show.ExitSuccess {
		t.Fatalf("code = %d, want %d; stderr=%q", code, show.ExitSuccess, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("expected no output, got %q", stderr.String())
	}
}

func TestRun_ReportsExecBitShebangAnnotationAndCollision(t *testing.T) {
	ws := setupWS(t)
	// non-exec + missing shebang + broken annotation, all in one file
	p := filepath.Join(ws, "human", "bad")
	if err := os.WriteFile(p, []byte("# ---\n# summary: unterminated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// collision: same name in shared and human
	mkTask(t, ws, "shared", "build", "#!/bin/sh\n")
	mkTask(t, ws, "human", "build", "#!/bin/sh\n")

	var stdout, stderr bytes.Buffer
	code, err := Run(show.AudienceHuman, nil, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if code != show.ExitGeneric {
		t.Fatalf("code = %d, want %d", code, show.ExitGeneric)
	}
	out := stderr.String()
	for _, want := range []string{"exec-bit", "shebang", "annotation", "collision: build"} {
		if !bytes.Contains([]byte(out), []byte(want)) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestRun_WorkspaceMissing(t *testing.T) {
	root := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(root)
	t.Cleanup(func() { os.Chdir(orig) })

	var stdout, stderr bytes.Buffer
	code, err := Run(show.AudienceHuman, nil, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if code != show.ExitWorkspaceMissing {
		t.Errorf("code = %d, want %d", code, show.ExitWorkspaceMissing)
	}
}

func TestRun_NameFilterScopesToOneTask(t *testing.T) {
	ws := setupWS(t)
	mkTask(t, ws, "shared", "build", "#!/bin/sh\n")     // clean
	writeNonExec := filepath.Join(ws, "shared", "lint")
	if err := os.WriteFile(writeNonExec, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code, err := Run(show.AudienceHuman, []string{"build"}, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	// build is clean; lint's exec-bit problem is out of scope -> success.
	if code != show.ExitSuccess {
		t.Errorf("code = %d, want %d; stderr=%q", code, show.ExitSuccess, stderr.String())
	}
}

func TestRun_NameNotFound(t *testing.T) {
	setupWS(t)
	var stdout, stderr bytes.Buffer
	code, err := Run(show.AudienceHuman, []string{"nope"}, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if code != show.ExitNotFound {
		t.Errorf("code = %d, want %d", code, show.ExitNotFound)
	}
}

func TestRun_AIEmitsEnvelope(t *testing.T) {
	ws := setupWS(t)
	mkTask(t, ws, "shared", "build", "#!/bin/sh\n# ---\n# summary: Build.\n# ---\n")

	var stdout, stderr bytes.Buffer
	code, err := Run(show.AudienceAI, nil, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if code != show.ExitSuccess {
		t.Fatalf("code = %d", code)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"kind":"validation"`)) {
		t.Errorf("expected validation envelope, got %q", stdout.String())
	}
}
