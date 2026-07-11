package show

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// RenderHumanTask writes the single-task detail view: path, blank line,
// indented summary, blank line, body. Body block is omitted entirely when
// body is empty (FR-003a). Summary line is omitted when no summary.
func RenderHumanTask(w io.Writer, e Entry) error {
	if _, err := fmt.Fprintln(w, e.Path); err != nil {
		return err
	}
	summary := strings.TrimSpace(e.Annotation.Summary)
	if summary != "" {
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "  %s\n", summary); err != nil {
			return err
		}
	}
	body := e.Annotation.Body
	if body != "" {
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, body); err != nil {
			return err
		}
	}
	if spec := compiledSpec(e.Path); spec != nil {
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
		invocation := "taskgate run " + runName(e.Path)
		if _, err := fmt.Fprintln(w, spec.UsageLine(invocation)); err != nil {
			return err
		}
	}
	return nil
}

// taskRowWidth is the printed width of a task row's "indent + name" prefix:
// two spaces per depth level plus the rune count of the basename.
func taskRowWidth(e Entry, depth int) int {
	return 2*depth + utf8.RuneCountInString(e.Name)
}

// hasSummary reports whether a task entry carries a non-empty trimmed summary.
func hasSummary(e Entry) bool {
	return e.Kind == EntryKindTask && strings.TrimSpace(e.Annotation.Summary) != ""
}

// summaryColumn returns the column (character offset) where summaries begin:
// the widest summary-bearing task row plus a two-space gap. When useEntryDepth
// is true each entry's own Depth is used (recursive tree); otherwise the fixed
// depth argument applies to every entry (single-level directory listing).
// Returns 0 when no row carries a summary, signalling "no column".
func summaryColumn(entries []Entry, depth int, useEntryDepth bool) int {
	widest := 0
	for _, e := range entries {
		if !hasSummary(e) {
			continue
		}
		d := depth
		if useEntryDepth {
			d = e.Depth
		}
		if w := taskRowWidth(e, d); w > widest {
			widest = w
		}
	}
	if widest == 0 {
		return 0
	}
	return widest + 2
}

// writeTreeRow writes a single indented tree row: two spaces per depth, then
// the basename. Directories get a trailing "/". Task rows with a summary pad
// the name out to col before printing the trimmed summary; task rows without
// a summary (or when col == 0) print the name alone.
func writeTreeRow(w io.Writer, e Entry, depth, col int) error {
	indent := strings.Repeat("  ", depth)
	if e.Kind == EntryKindDirectory {
		_, err := fmt.Fprintf(w, "%s%s/\n", indent, e.Name)
		return err
	}
	summary := strings.TrimSpace(e.Annotation.Summary)
	if summary == "" || col == 0 {
		_, err := fmt.Fprintf(w, "%s%s\n", indent, e.Name)
		return err
	}
	pad := col - taskRowWidth(e, depth)
	if pad < 1 {
		pad = 1
	}
	_, err := fmt.Fprintf(w, "%s%s%s%s\n", indent, e.Name, strings.Repeat(" ", pad), summary)
	return err
}

// RenderHumanTree writes the recursive listing as an indented tree, one row
// per entry, indented by Entry.Depth, with summaries in a single aligned
// column spanning the whole tree.
func RenderHumanTree(w io.Writer, entries []Entry) error {
	col := summaryColumn(entries, 0, true)
	for _, e := range entries {
		if err := writeTreeRow(w, e, e.Depth, col); err != nil {
			return err
		}
	}
	return nil
}

// RenderHumanDirectory writes the directory-target view: the directory's real
// path, a blank line, then its immediate children as one-level tree rows with
// summaries aligned within this listing. Directories carry no summary/body.
func RenderHumanDirectory(w io.Writer, target ResolvedTarget) error {
	if _, err := fmt.Fprintln(w, target.Entry.Path); err != nil {
		return err
	}
	if len(target.Children) > 0 {
		col := summaryColumn(target.Children, 1, false)
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
		for _, c := range target.Children {
			if err := writeTreeRow(w, c, 1, col); err != nil {
				return err
			}
		}
	}
	return nil
}
