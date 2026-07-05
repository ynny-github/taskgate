package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ynny-github/taskgate/taskgate/internal/usage"
)

func TestAIUsageCmd_PrintsGuide(t *testing.T) {
	var buf bytes.Buffer
	root := newRootCmd()
	root.SetArgs([]string{"ai", "usage"})
	root.SetOut(&buf)
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.String() != usage.Guide() {
		t.Errorf("output does not match usage.Guide()")
	}
}

func TestAIUsageCmd_RejectsArgs(t *testing.T) {
	var errBuf bytes.Buffer
	root := newRootCmd()
	root.SetArgs([]string{"ai", "usage", "extra"})
	root.SetErr(&errBuf)
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when an argument is supplied, got nil")
	}
}

func TestAIUsageCmd_MentionsShowAndRun(t *testing.T) {
	var buf bytes.Buffer
	root := newRootCmd()
	root.SetArgs([]string{"ai", "usage"})
	root.SetOut(&buf)
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "taskgate ai show") || !strings.Contains(out, "taskgate ai run") {
		t.Errorf("guide should mention 'taskgate ai show' and 'taskgate ai run'")
	}
}
