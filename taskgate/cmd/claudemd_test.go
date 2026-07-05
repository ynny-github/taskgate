package cmd

import (
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
