package validate

import (
	"bytes"
	"os"

	"github.com/ynny-github/taskgate/taskgate/internal/annotation"
)

// discovered is one file found during the bucket walk.
type discovered struct {
	absPath     string
	displayPath string
	logicalName string
	isIndex     bool
}

// checkFile runs the per-file rules against d. Task files are checked for
// execute bit, shebang, and annotation format; _index files are checked for
// annotation format only (ADR-0002: never executed).
func checkFile(d discovered) ([]Finding, error) {
	var out []Finding

	info, err := os.Stat(d.absPath)
	if err != nil {
		return nil, err
	}
	if !d.isIndex && info.Mode()&0o111 == 0 {
		out = append(out, Finding{
			Rule:    RuleExecBit,
			Path:    d.displayPath,
			Message: "task file is not executable",
			logical: d.logicalName,
		})
	}

	data, err := os.ReadFile(d.absPath)
	if err != nil {
		return nil, err
	}
	if !d.isIndex && !bytes.HasPrefix(data, []byte("#!")) {
		out = append(out, Finding{
			Rule:    RuleShebang,
			Path:    d.displayPath,
			Message: "missing shebang (first line must start with #!)",
			logical: d.logicalName,
		})
	}

	_, diag, err := annotation.ParseStrict(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if diag != nil {
		out = append(out, Finding{
			Rule:    RuleAnnotation,
			Path:    d.displayPath,
			Message: diag.Reason,
			logical: d.logicalName,
		})
	}

	return out, nil
}
