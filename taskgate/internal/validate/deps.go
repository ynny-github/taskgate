package validate

import (
	"bytes"
	"fmt"
	"os"
	"sort"

	"github.com/ynny-github/taskgate/taskgate/internal/annotation"
	"github.com/ynny-github/taskgate/taskgate/internal/show"
)

// depTask is a resolved task in the audience view: its file plus its declared
// edges (before ++ after), used for reference/cycle checks.
type depTask struct {
	d     discovered
	edges []string
}

// detectDeps statically checks before/after across the audience's view
// (bucket ++ shared, bucket-first). It reports malformed lists, unknown
// references, non-executable dependency targets, and cycles.
func detectDeps(audience show.Audience, perFiles map[string][]discovered) ([]Finding, error) {
	bucket := "human"
	if audience == show.AudienceAI {
		bucket = "ai"
	}

	// Merged resolution map: logical name -> discovered, bucket wins over shared.
	resolve := map[string]discovered{}
	for _, d := range perFiles["shared"] {
		resolve[d.logicalName] = d
	}
	for _, d := range perFiles[bucket] {
		resolve[d.logicalName] = d
	}

	var findings []Finding
	tasks := map[string]depTask{}
	for _, name := range sortedKeys(resolve) {
		d := resolve[name]
		data, err := os.ReadFile(d.absPath)
		if err != nil {
			return nil, err
		}
		deps, diag, err := annotation.ParseDeps(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		if diag != nil {
			findings = append(findings, Finding{
				Rule: RuleDepMalformed, Path: d.displayPath, Message: diag.Reason, logical: d.logicalName,
			})
			continue // cannot trust this task's edges
		}
		var edges []string
		for _, ref := range append(append([]string{}, deps.Before...), deps.After...) {
			target, ok := resolve[ref]
			if !ok {
				findings = append(findings, Finding{
					Rule: RuleDepUnknown, Path: d.displayPath,
					Message: fmt.Sprintf("dependency %q not found in the audience view", ref),
					logical: d.logicalName,
				})
				continue
			}
			if info, err := os.Stat(target.absPath); err == nil && info.Mode()&0o111 == 0 {
				findings = append(findings, Finding{
					Rule: RuleDepNotExec, Path: d.displayPath,
					Message: fmt.Sprintf("dependency %q is not executable", ref),
					logical: d.logicalName,
				})
				continue
			}
			edges = append(edges, ref)
		}
		tasks[name] = depTask{d: d, edges: edges}
	}

	findings = append(findings, detectCycles(tasks)...)
	return findings, nil
}

// detectCycles runs DFS coloring over the resolved edges and emits one
// dep-cycle finding per task that participates in a back-edge.
func detectCycles(tasks map[string]depTask) []Finding {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := map[string]int{}
	var out []Finding
	var seen = map[string]bool{}

	var dfs func(name string)
	dfs = func(name string) {
		color[name] = gray
		for _, next := range tasks[name].edges {
			switch color[next] {
			case gray:
				if !seen[next] {
					seen[next] = true
					out = append(out, Finding{
						Rule: RuleDepCycle, Path: tasks[next].d.displayPath,
						Message: "task participates in a dependency cycle", logical: tasks[next].d.logicalName,
					})
				}
			case white:
				dfs(next)
			}
		}
		color[name] = black
	}
	for _, name := range sortedTaskKeys(tasks) {
		if color[name] == white {
			dfs(name)
		}
	}
	return out
}

func sortedKeys(m map[string]discovered) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedTaskKeys(m map[string]depTask) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
