package validate

import (
	"fmt"
	"io"
	"strings"

	"github.com/ynny-github/taskgate/taskgate/internal/show"
)

// renderHuman writes one line per finding to stderr and returns the exit code.
// Zero findings produce no output and ExitSuccess.
func renderHuman(stderr io.Writer, findings []Finding) (int, error) {
	if len(findings) == 0 {
		return show.ExitSuccess, nil
	}
	for _, f := range findings {
		if f.Rule == RuleCollision {
			fmt.Fprintf(stderr, "collision: %s: %s\n", f.Name, strings.Join(f.Paths, ", "))
			continue
		}
		fmt.Fprintf(stderr, "%s: %s: %s\n", f.Path, f.Rule, f.Message)
	}
	return show.ExitGeneric, nil
}

func renderWorkspaceMissing(a show.Audience, stdout, stderr io.Writer) (int, error) {
	const msg = ".taskgate/ not found"
	if a == show.AudienceAI {
		return show.ExitWorkspaceMissing, writeAIError(stdout, "workspace_missing", msg)
	}
	fmt.Fprintln(stderr, msg)
	return show.ExitWorkspaceMissing, nil
}

func renderNotFound(a show.Audience, rep show.NotFoundReport, stdout, stderr io.Writer) (int, error) {
	msg := fmt.Sprintf("task %q not found", rep.Name)
	if a == show.AudienceAI {
		return show.ExitNotFound, writeAIError(stdout, "not_found", msg)
	}
	fmt.Fprintln(stderr, msg)
	return show.ExitNotFound, nil
}

func renderInvalidInput(a show.Audience, rep show.InvalidInputReport, stdout, stderr io.Writer) (int, error) {
	msg := fmt.Sprintf("invalid name %q: %s", rep.Input, rep.Reason)
	if a == show.AudienceAI {
		return show.ExitInvalidInput, writeAIError(stdout, "invalid_input", msg)
	}
	fmt.Fprintln(stderr, msg)
	return show.ExitInvalidInput, nil
}
