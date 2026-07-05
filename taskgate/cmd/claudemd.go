package cmd

import (
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
		sep := "\n"
		if !strings.HasSuffix(existing, "\n") {
			sep = "\n\n"
		} else if !strings.HasSuffix(existing, "\n\n") {
			sep = "\n"
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
