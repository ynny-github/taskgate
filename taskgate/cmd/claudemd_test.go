package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpsertClaudeMdBlock_AppendsToNonEmpty(t *testing.T) {
	existing := "# My Project\n\nSome notes.\n"
	got, changed := upsertClaudeMdBlock(existing)
	if !changed {
		t.Fatal("expected changed=true when block is absent")
	}
	if !strings.HasPrefix(got, existing) {
		t.Errorf("existing content must be preserved as a prefix, got: %q", got)
	}
	if !strings.Contains(got, claudeMdBeginMarker) || !strings.Contains(got, claudeMdEndMarker) {
		t.Errorf("output must contain both markers, got: %q", got)
	}
	if !strings.Contains(got, "taskgate ai usage") {
		t.Errorf("output must contain the pointer body, got: %q", got)
	}
}

func TestUpsertClaudeMdBlock_ReplacesExistingBlockOnly(t *testing.T) {
	existing := "# Top\n\n" +
		claudeMdBeginMarker + "\n\nOLD BODY\n\n" + claudeMdEndMarker + "\n\n# Bottom\n"
	got, changed := upsertClaudeMdBlock(existing)
	if !changed {
		t.Fatal("expected changed=true when stored block differs")
	}
	if strings.Contains(got, "OLD BODY") {
		t.Errorf("old block body should be replaced, got: %q", got)
	}
	if !strings.Contains(got, "# Top") || !strings.Contains(got, "# Bottom") {
		t.Errorf("content outside markers must be preserved, got: %q", got)
	}
	if strings.Count(got, claudeMdBeginMarker) != 1 {
		t.Errorf("must not duplicate the block, got %d begin markers", strings.Count(got, claudeMdBeginMarker))
	}
}

func TestUpsertClaudeMdBlock_IdempotentWhenCurrent(t *testing.T) {
	first, _ := upsertClaudeMdBlock("# Top\n")
	second, changed := upsertClaudeMdBlock(first)
	if changed {
		t.Errorf("expected changed=false on already-current content")
	}
	if second != first {
		t.Errorf("re-running upsert must be a no-op, got different content")
	}
}

func TestUpsertClaudeMdBlock_NoDoubleBlankWhenTrailingBlankLine(t *testing.T) {
	existing := "text\n\n"
	got, changed := upsertClaudeMdBlock(existing)
	if !changed {
		t.Fatal("expected changed=true when block is absent")
	}
	want := existing + renderClaudeMdBlock()
	if got != want {
		t.Errorf("separator should be empty when existing ends with a blank line\n got: %q\nwant: %q", got, want)
	}
	second, changed2 := upsertClaudeMdBlock(got)
	if changed2 {
		t.Errorf("second upsert must be a no-op, got changed=true")
	}
	if second != got {
		t.Errorf("second upsert must not modify content")
	}
}

func TestUpsertClaudeMdBlock_EmptyInput(t *testing.T) {
	got, changed := upsertClaudeMdBlock("")
	if !changed {
		t.Fatal("expected changed=true for empty input")
	}
	if got != renderClaudeMdBlock() {
		t.Errorf("empty input must yield exactly the rendered block\n got: %q\nwant: %q", got, renderClaudeMdBlock())
	}
}

func TestEnsureClaudeMdPointer_CreatesRootWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	path, action, err := ensureClaudeMdPointer(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action != "created" {
		t.Errorf("action = %q, want \"created\"", action)
	}
	if path != "CLAUDE.md" {
		t.Errorf("path = %q, want \"CLAUDE.md\"", path)
	}
	data, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("reading created file: %v", err)
	}
	if !strings.Contains(string(data), claudeMdBeginMarker) {
		t.Errorf("created CLAUDE.md missing managed block")
	}
}

func TestEnsureClaudeMdPointer_PrefersDotClaudeWhenPresent(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".claude", "CLAUDE.md"), []byte("# existing\n"), 0644); err != nil {
		t.Fatal(err)
	}
	path, action, err := ensureClaudeMdPointer(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != filepath.Join(".claude", "CLAUDE.md") {
		t.Errorf("path = %q, want .claude/CLAUDE.md", path)
	}
	if action != "updated" {
		t.Errorf("action = %q, want \"updated\"", action)
	}
	if _, err := os.Stat(filepath.Join(dir, "CLAUDE.md")); !os.IsNotExist(err) {
		t.Errorf("root CLAUDE.md must not be created when .claude/CLAUDE.md exists")
	}
}

func TestEnsureClaudeMdPointer_UnchangedOnSecondRun(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := ensureClaudeMdPointer(dir); err != nil {
		t.Fatal(err)
	}
	_, action, err := ensureClaudeMdPointer(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action != "unchanged" {
		t.Errorf("action = %q, want \"unchanged\" on idempotent re-run", action)
	}
}
