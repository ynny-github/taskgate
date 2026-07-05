package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ynny-github/taskgate/taskgate/internal/show"
)

func chdirTemp(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
	return root
}

func TestValidateCmd_CleanTreeExitsZero(t *testing.T) {
	root := chdirTemp(t)
	shared := filepath.Join(root, ".taskgate", "shared")
	if err := os.MkdirAll(shared, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shared, "build"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	rootCmd := newRootCmd()
	rootCmd.SetArgs([]string{"validate"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateCmd_FindingReturnsExitError(t *testing.T) {
	root := chdirTemp(t)
	shared := filepath.Join(root, ".taskgate", "shared")
	if err := os.MkdirAll(shared, 0o755); err != nil {
		t.Fatal(err)
	}
	// non-executable task -> exec-bit finding -> ExitError{1}
	if err := os.WriteFile(filepath.Join(shared, "build"), []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rootCmd := newRootCmd()
	rootCmd.SetArgs([]string{"validate"})
	var stderr bytes.Buffer
	rootCmd.SetErr(&stderr)
	err := rootCmd.Execute()

	var exitErr *show.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected *show.ExitError, got %v", err)
	}
	if exitErr.Code != show.ExitGeneric {
		t.Errorf("code = %d, want %d", exitErr.Code, show.ExitGeneric)
	}
}

func TestAIValidateCmd_EmitsEnvelope(t *testing.T) {
	root := chdirTemp(t)
	shared := filepath.Join(root, ".taskgate", "shared")
	if err := os.MkdirAll(shared, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shared, "build"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	rootCmd := newRootCmd()
	var stdout bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetArgs([]string{"ai", "validate"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"kind":"validation"`)) {
		t.Errorf("expected validation envelope, got %q", stdout.String())
	}
}
