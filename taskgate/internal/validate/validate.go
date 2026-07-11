package validate

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/ynny-github/taskgate/taskgate/internal/show"
)

var buckets = []string{"human", "ai", "shared"}

// Run is the package entry point for `taskgate validate` / `taskgate ai
// validate`. cmd/validate.go thin-wires into it with the audience selector.
func Run(audience show.Audience, args []string, stdout, stderr io.Writer) (int, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return show.ExitGeneric, err
	}
	ws, err := show.WorkspaceDir(cwd)
	if err != nil {
		if errors.Is(err, show.ErrWorkspaceMissing) {
			return renderWorkspaceMissing(audience, stdout, stderr)
		}
		return show.ExitGeneric, err
	}

	var name string
	switch len(args) {
	case 0:
	case 1:
		name = args[0]
		if rep := show.ValidateName(name); rep != nil {
			return renderInvalidInput(audience, *rep, stdout, stderr)
		}
	default:
		return show.ExitGeneric, fmt.Errorf("validate accepts at most one positional argument")
	}

	perFiles := map[string][]discovered{}
	perSlots := map[string]map[string]string{}
	for _, b := range buckets {
		files, slots, err := discoverBucket(ws, b)
		if err != nil {
			return show.ExitGeneric, err
		}
		perFiles[b] = files
		perSlots[b] = slots
	}

	var findings []Finding
	for _, b := range buckets {
		for _, d := range perFiles[b] {
			fs, err := checkFile(d)
			if err != nil {
				return show.ExitGeneric, err
			}
			findings = append(findings, fs...)
		}
	}
	findings = append(findings, detectCollisions(perSlots)...)

	depFindings, err := detectDeps(audience, perFiles)
	if err != nil {
		return show.ExitGeneric, err
	}
	findings = append(findings, depFindings...)

	if name != "" {
		if !nameExists(name, perFiles, perSlots) {
			return renderNotFound(audience, show.NotFoundReport{
				Name:     name,
				Searched: []string{".taskgate/human", ".taskgate/ai", ".taskgate/shared"},
			}, stdout, stderr)
		}
		findings = filterByName(findings, name)
	}

	sortFindings(findings)

	if audience == show.AudienceAI {
		return renderAI(stdout, findings)
	}
	return renderHuman(stderr, findings)
}

// nameExists reports whether the logical name occupies any file or slot across
// buckets (a task file, an _index's directory, or a subdirectory).
func nameExists(name string, perFiles map[string][]discovered, perSlots map[string]map[string]string) bool {
	for _, b := range buckets {
		for _, d := range perFiles[b] {
			if d.logicalName == name {
				return true
			}
		}
		if _, ok := perSlots[b][name]; ok {
			return true
		}
	}
	return false
}

func filterByName(findings []Finding, name string) []Finding {
	out := findings[:0:0]
	for _, f := range findings {
		if f.logical == name {
			out = append(out, f)
		}
	}
	return out
}

// sortFindings orders findings deterministically: by Path, then Rule, then
// Name, then joined Paths. Collision findings (empty Path) sort ahead of
// file findings and are ordered by Name.
func sortFindings(findings []Finding) {
	sort.SliceStable(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Rule != b.Rule {
			return a.Rule < b.Rule
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return strings.Join(a.Paths, ",") < strings.Join(b.Paths, ",")
	})
}
