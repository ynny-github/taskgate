// Package usage holds the single source of truth for the AI-facing
// taskgate usage guide and the short pointer injected into CLAUDE.md.
package usage

import _ "embed"

//go:embed guide.md
var guide string

// Guide returns the full Markdown usage guide printed by `taskgate ai usage`.
func Guide() string {
	return guide
}

// Pointer returns the Markdown body placed between the CLAUDE.md managed-block
// markers. It does not include the markers themselves.
func Pointer() string {
	return `## taskgate

This project uses **taskgate** as its task runner.
Before running project tasks, run ` + "`taskgate ai usage`" + ` to learn how to
discover and execute tasks safely.`
}
