// taskgate/cmd/exec.go
package cmd

import (
	"errors"
	"fmt"
	"io"
	"os/exec"

	"github.com/ynny-github/taskgate/taskgate/internal/show"
)

// Run is the embeddable entry point. main.go calls it once; the testscript
// suite calls it via testscript.RunMain so each scenario observes a real
// stdout/stderr writer and a real exit code.
//
// args is everything after the program name (i.e. os.Args[1:]).
func Run(args []string, stdout, stderr io.Writer) int {
	root := newRootCmd()
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	if err := root.Execute(); err != nil {
		var execErr *exec.ExitError
		if errors.As(err, &execErr) {
			return execErr.ExitCode()
		}
		var showErr *show.ExitError
		if errors.As(err, &showErr) {
			return showErr.Code
		}
		fmt.Fprintln(stderr, "taskgate:", err)
		return 1
	}
	return 0
}
