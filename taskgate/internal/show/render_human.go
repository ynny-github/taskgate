package show

import (
	"fmt"
	"io"
	"strings"
)

// RenderHumanListing writes one row per entry in the form
// `<real path>  <summary>`. When summary is empty the row is path-only.
func RenderHumanListing(w io.Writer, entries []Entry) error {
	for _, e := range entries {
		if err := writeListingRow(w, e); err != nil {
			return err
		}
	}
	return nil
}

func writeListingRow(w io.Writer, e Entry) error {
	path := displayPath(e)
	summary := strings.TrimSpace(e.Annotation.Summary)
	if summary == "" {
		_, err := fmt.Fprintln(w, path)
		return err
	}
	_, err := fmt.Fprintf(w, "%s\t%s\n", path, summary)
	return err
}

// displayPath returns the path string used in human output. Directory
// entries get a trailing "/" so the listing visually groups them.
func displayPath(e Entry) string {
	if e.Kind == EntryKindDirectory {
		return e.Path + "/"
	}
	return e.Path
}

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
	return nil
}

// writeTreeRow writes a single indented tree row: two spaces per depth,
// then the basename. Directories get a trailing "/"; task rows append a
// tab and the trimmed summary when one is present.
func writeTreeRow(w io.Writer, e Entry, depth int) error {
	indent := strings.Repeat("  ", depth)
	if e.Kind == EntryKindDirectory {
		_, err := fmt.Fprintf(w, "%s%s/\n", indent, e.Name)
		return err
	}
	summary := strings.TrimSpace(e.Annotation.Summary)
	if summary == "" {
		_, err := fmt.Fprintf(w, "%s%s\n", indent, e.Name)
		return err
	}
	_, err := fmt.Fprintf(w, "%s%s\t%s\n", indent, e.Name, summary)
	return err
}

// RenderHumanTree writes the recursive listing as an indented tree, one row
// per entry, indented by Entry.Depth.
func RenderHumanTree(w io.Writer, entries []Entry) error {
	for _, e := range entries {
		if err := writeTreeRow(w, e, e.Depth); err != nil {
			return err
		}
	}
	return nil
}

// RenderHumanDirectory writes the directory-target view: the directory's
// real path, a blank line, then its immediate children as one-level tree
// rows. Directories carry no summary/body.
func RenderHumanDirectory(w io.Writer, target ResolvedTarget) error {
	if _, err := fmt.Fprintln(w, target.Entry.Path); err != nil {
		return err
	}
	if len(target.Children) > 0 {
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
		for _, c := range target.Children {
			if err := writeTreeRow(w, c, 1); err != nil {
				return err
			}
		}
	}
	return nil
}
