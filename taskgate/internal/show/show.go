// Package show implements the taskgate show / taskgate ai show subcommands.
// Resolution happens against the merged audience+shared view; rendering is
// fan-out per audience.
package show

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// Run is the package entry point. cmd/show.go and cmd/ai_show.go thin-wire
// into this function with their audience selector. exitCode follows the
// contract in contracts/cli.md.
func Run(audience Audience, args []string, stdout, stderr io.Writer) (exitCode int, err error) {
	cwd, err := os.Getwd()
	if err != nil {
		return ExitGeneric, err
	}
	ws, err := WorkspaceDir(cwd)
	if err != nil {
		if errors.Is(err, ErrWorkspaceMissing) {
			return renderWorkspaceMissing(audience, stdout, stderr)
		}
		return ExitGeneric, err
	}

	switch len(args) {
	case 0:
		return runRoot(audience, ws, stdout, stderr)
	case 1:
		return runNamed(audience, ws, args[0], stdout, stderr)
	default:
		return ExitGeneric, fmt.Errorf("show accepts at most one positional argument")
	}
}

func runRoot(audience Audience, ws string, stdout, stderr io.Writer) (int, error) {
	entries, col, err := ResolveTree(audience, ws)
	if err != nil {
		return ExitGeneric, err
	}
	if col != nil {
		return renderCollision(audience, *col, stdout, stderr)
	}
	if audience == AudienceAI {
		return ExitSuccess, renderAIRootListing(stdout, entries)
	}
	emitNotices(stderr, entries)
	return ExitSuccess, RenderHumanTree(stdout, entries)
}

// emitNotices writes a "<path>: <note>" line to stderr for each entry
// whose Note is non-empty (T038/T039). Only used by the human form.
func emitNotices(stderr io.Writer, entries []Entry) {
	for _, e := range entries {
		if e.Note != "" {
			fmt.Fprintf(stderr, "%s: %s\n", e.Path, e.Note)
		}
	}
}

func runNamed(audience Audience, ws, name string, stdout, stderr io.Writer) (int, error) {
	if rep := ValidateName(name); rep != nil {
		return renderInvalidInput(audience, *rep, stdout, stderr)
	}
	target, col, nf, err := ResolveName(audience, ws, name)
	if err != nil {
		return ExitGeneric, err
	}
	if col != nil {
		return renderCollision(audience, *col, stdout, stderr)
	}
	if nf != nil {
		return renderNotFound(audience, *nf, stdout, stderr)
	}
	if audience == AudienceAI {
		return ExitSuccess, renderAITarget(stdout, *target)
	}
	switch target.Kind {
	case EntryKindTask:
		emitNotices(stderr, []Entry{target.Entry})
		return ExitSuccess, RenderHumanTask(stdout, target.Entry)
	case EntryKindDirectory:
		emitNotices(stderr, []Entry{target.Entry})
		emitNotices(stderr, target.Children)
		return ExitSuccess, RenderHumanDirectory(stdout, *target)
	default:
		return ExitGeneric, fmt.Errorf("unknown target kind")
	}
}

func renderInvalidInput(audience Audience, rep InvalidInputReport, stdout, stderr io.Writer) (int, error) {
	if audience == AudienceAI {
		return ExitInvalidInput, renderAIError(stdout, errorEnvelope{
			Kind:    "error",
			Err:     "invalid_input",
			Message: "taskgate ai show accepts run-style names, not filesystem paths",
			Input:   rep.Input,
			Reason:  rep.Reason,
		})
	}
	fmt.Fprintln(stderr, "taskgate show accepts run-style names (bare or slash-separated), not filesystem paths")
	return ExitInvalidInput, nil
}

func renderNotFound(audience Audience, rep NotFoundReport, stdout, stderr io.Writer) (int, error) {
	if audience == AudienceAI {
		return ExitNotFound, renderAIError(stdout, errorEnvelope{
			Kind:     "error",
			Err:      "not_found",
			Message:  fmt.Sprintf("%q not found in %s", rep.Name, strings.Join(rep.Searched, " or ")),
			Name:     rep.Name,
			Searched: rep.Searched,
		})
	}
	fmt.Fprintf(stderr, "%q not found in %s\n", rep.Name, strings.Join(rep.Searched, " or "))
	return ExitNotFound, nil
}

func renderAITarget(w io.Writer, target ResolvedTarget) error {
	switch target.Kind {
	case EntryKindTask:
		env := taskEnvelope{
			Kind:     "task",
			Name:     runName(target.Entry.Path),
			Path:     target.Entry.Path,
			Summary:  summaryPtr(target.Entry.Annotation.Summary),
			Body:     target.Entry.Annotation.Body,
			Audience: "ai",
		}
		return renderAI(w, env)
	case EntryKindDirectory:
		env := directoryEnvelope{
			Kind:     "directory",
			Name:     runName(target.Entry.Path),
			Path:     target.Entry.Path,
			Audience: "ai",
			Entries:  childRecords(target.Children),
		}
		return renderAI(w, env)
	}
	return fmt.Errorf("unknown target kind")
}

func renderCollision(audience Audience, col CollisionReport, stdout, stderr io.Writer) (int, error) {
	if audience == AudienceAI {
		return ExitCollision, renderAIError(stdout, errorEnvelope{
			Kind:    "error",
			Err:     "collision",
			Message: fmt.Sprintf("name %q collides across audience bucket and shared", col.Name),
			Name:    col.Name,
			Paths:   col.Paths,
		})
	}
	fmt.Fprintf(stderr, "name %q collides: %s\n", col.Name, strings.Join(col.Paths, ", "))
	return ExitCollision, nil
}

func renderWorkspaceMissing(audience Audience, stdout, stderr io.Writer) (int, error) {
	if audience == AudienceAI {
		return ExitWorkspaceMissing, renderAIError(stdout, errorEnvelope{
			Kind:    "error",
			Err:     "workspace_missing",
			Message: ".taskgate/ not found at the current project root",
		})
	}
	fmt.Fprintln(stderr, ".taskgate/ not found")
	return ExitWorkspaceMissing, nil
}
