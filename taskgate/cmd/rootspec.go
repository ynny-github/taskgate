package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/ynny-github/taskgate/taskgate/internal/annotation"
	"github.com/ynny-github/taskgate/taskgate/internal/cliparse"
)

// applyRootSpec parses the root target's CLI spec (if any) against scriptArgs.
// See the plan's Task 5 interface block for the return-value contract.
func applyRootSpec(rootPath, invocation string, scriptArgs []string, stdout, stderr io.Writer) (adds, forwarded []string, handled bool, err error) {
	data, err := os.ReadFile(rootPath)
	if err != nil {
		return nil, nil, false, err
	}
	raw, diag, err := annotation.ParseArgSpec(bytes.NewReader(data))
	if err != nil {
		return nil, nil, false, err
	}
	if diag != nil {
		return nil, nil, false, fmt.Errorf("invalid CLI spec: %s", diag.Reason)
	}
	spec, probs := cliparse.Compile(raw)
	if len(probs) > 0 {
		return nil, nil, false, fmt.Errorf("invalid CLI spec: %s", probs[0])
	}
	if spec == nil { // no spec declared → raw passthrough
		return nil, scriptArgs, false, nil
	}
	res, uerr := spec.Parse(scriptArgs)
	if uerr != nil {
		fmt.Fprintf(stderr, "taskgate: %s\n%s\n", uerr.Reason, spec.UsageLine(invocation))
		return nil, nil, false, &exitError{code: 2}
	}
	if res.Help {
		block, _ := annotation.Parse(bytes.NewReader(data))
		fmt.Fprint(stdout, spec.Help(invocation, block.Summary, block.Body))
		return nil, nil, true, nil
	}
	for k, v := range res.Env {
		adds = append(adds, k+"="+v)
	}
	return adds, nil, false, nil
}
