package validate

import (
	"bytes"
	"os"

	"github.com/ynny-github/taskgate/taskgate/internal/annotation"
	"github.com/ynny-github/taskgate/taskgate/internal/cliparse"
	"github.com/ynny-github/taskgate/taskgate/internal/show"
)

// detectSpec statically lints each task's args/flags CLI declaration across the
// audience's view (bucket + shared). It reports a malformed block (bad YAML
// shape) and semantic problems (bad flag names, required+default, etc.).
func detectSpec(audience show.Audience, perFiles map[string][]discovered) ([]Finding, error) {
	bucket := "human"
	if audience == show.AudienceAI {
		bucket = "ai"
	}
	resolve := map[string]discovered{}
	for _, d := range perFiles["shared"] {
		resolve[d.logicalName] = d
	}
	for _, d := range perFiles[bucket] {
		resolve[d.logicalName] = d
	}

	var findings []Finding
	for _, name := range sortedKeys(resolve) {
		d := resolve[name]
		data, err := os.ReadFile(d.absPath)
		if err != nil {
			return nil, err
		}
		raw, diag, err := annotation.ParseArgSpec(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		if diag != nil {
			findings = append(findings, Finding{
				Rule: RuleSpecMalformed, Path: d.displayPath, Message: diag.Reason, logical: d.logicalName,
			})
			continue
		}
		_, probs := cliparse.Compile(raw)
		for _, p := range probs {
			findings = append(findings, Finding{
				Rule: RuleSpecInvalid, Path: d.displayPath, Message: p, logical: d.logicalName,
			})
		}
	}
	return findings, nil
}
