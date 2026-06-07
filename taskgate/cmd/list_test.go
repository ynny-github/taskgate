// taskgate/cmd/list_test.go
package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeTask(t *testing.T, tmp, subdir, name string) {
	t.Helper()
	dir := filepath.Join(tmp, ".taskgate", subdir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\necho hi"), 0755); err != nil {
		t.Fatal(err)
	}
}

func TestListCmd_NoTaskgateDir(t *testing.T) {
	tmp := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	root := newRootCmd()
	root.SetArgs([]string{"list"})
	err = root.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), ".taskgate/") {
		t.Errorf("expected error to mention .taskgate/, got: %q", err.Error())
	}
}

func TestListCmd_EmptyDirs(t *testing.T) {
	tmp := t.TempDir()
	for _, sub := range []string{"human", "shared"} {
		if err := os.MkdirAll(filepath.Join(tmp, ".taskgate", sub), 0755); err != nil {
			t.Fatal(err)
		}
	}
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	var buf bytes.Buffer
	root := newRootCmd()
	root.SetArgs([]string{"list"})
	root.SetOut(&buf)
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.String() != "" {
		t.Errorf("expected empty output, got: %q", buf.String())
	}
}

func TestListCmd_HumanAndShared(t *testing.T) {
	tmp := t.TempDir()
	makeTask(t, tmp, "human", "build")
	makeTask(t, tmp, "shared", "lint")
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	var buf bytes.Buffer
	root := newRootCmd()
	root.SetArgs([]string{"list"})
	root.SetOut(&buf)
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "human/build\nshared/lint\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

func TestListCmd_AlphabeticalOrder(t *testing.T) {
	tmp := t.TempDir()
	makeTask(t, tmp, "human", "test")
	makeTask(t, tmp, "human", "build")
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	var buf bytes.Buffer
	root := newRootCmd()
	root.SetArgs([]string{"list"})
	root.SetOut(&buf)
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "human/build\nhuman/test\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

func TestListCmd_SkipsSubdirs(t *testing.T) {
	tmp := t.TempDir()
	makeTask(t, tmp, "human", "build")
	if err := os.MkdirAll(filepath.Join(tmp, ".taskgate", "human", "subdir"), 0755); err != nil {
		t.Fatal(err)
	}
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	var buf bytes.Buffer
	root := newRootCmd()
	root.SetArgs([]string{"list"})
	root.SetOut(&buf)
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "human/build\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

func TestListCmd_MissingSubdirsSkipped(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".taskgate"), 0755); err != nil {
		t.Fatal(err)
	}
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	var buf bytes.Buffer
	root := newRootCmd()
	root.SetArgs([]string{"list"})
	root.SetOut(&buf)
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.String() != "" {
		t.Errorf("expected empty output, got: %q", buf.String())
	}
}
