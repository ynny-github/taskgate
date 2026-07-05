package cmd

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ynny-github/taskgate/taskgate/internal/usage"
)

const (
	claudeMdBeginMarker = "<!-- taskgate:begin -->"
	claudeMdEndMarker   = "<!-- taskgate:end -->"
)

// renderClaudeMdBlock returns the full managed block, including both markers
// and a trailing newline.
func renderClaudeMdBlock() string {
	return claudeMdBeginMarker + "\n\n" + usage.Pointer() + "\n\n" + claudeMdEndMarker + "\n"
}

// upsertClaudeMdBlock returns existing with the taskgate managed block inserted
// or replaced. When the block is already present and byte-identical, it returns
// existing unchanged with changed=false. Content outside the markers is never
// modified.
func upsertClaudeMdBlock(existing string) (string, bool) {
	block := renderClaudeMdBlock()

	begin := strings.Index(existing, claudeMdBeginMarker)
	if begin == -1 {
		// No block yet: append, ensuring a blank line separates prior content.
		if existing == "" {
			return block, true
		}
		var sep string
		switch {
		case strings.HasSuffix(existing, "\n\n"):
			sep = ""
		case strings.HasSuffix(existing, "\n"):
			sep = "\n"
		default:
			sep = "\n\n"
		}
		return existing + sep + block, true
	}

	endIdx := strings.Index(existing, claudeMdEndMarker)
	if endIdx == -1 {
		// Corrupt/half-written block: treat everything from begin as the block.
		endIdx = len(existing) - len(claudeMdEndMarker)
	}
	after := endIdx + len(claudeMdEndMarker)
	// Consume a single trailing newline after the end marker so replacement
	// does not accumulate blank lines.
	trailing := existing[after:]
	if strings.HasPrefix(trailing, "\n") {
		trailing = trailing[1:]
	}

	updated := existing[:begin] + block + trailing
	if updated == existing {
		return existing, false
	}
	return updated, true
}

// ensureClaudeMdPointer writes/updates the taskgate managed block in the
// project's CLAUDE.md under dir. If dir/.claude/CLAUDE.md exists it is targeted;
// otherwise dir/CLAUDE.md is used (created when missing). The returned path is
// relative to dir. action is "created", "updated", or "unchanged".
func ensureClaudeMdPointer(dir string) (string, string, error) {
	rel := "CLAUDE.md"
	if _, err := os.Stat(filepath.Join(dir, ".claude", "CLAUDE.md")); err == nil {
		rel = filepath.Join(".claude", "CLAUDE.md")
	}
	full := filepath.Join(dir, rel)

	existingBytes, err := os.ReadFile(full)
	created := false
	if err != nil {
		if !os.IsNotExist(err) {
			return rel, "", err
		}
		created = true
	}

	updated, changed := upsertClaudeMdBlock(string(existingBytes))
	if !changed {
		return rel, "unchanged", nil
	}
	if err := os.WriteFile(full, []byte(updated), 0644); err != nil {
		return rel, "", err
	}
	if created {
		return rel, "created", nil
	}
	return rel, "updated", nil
}
