// taskgate/main.go
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/ynny-github/taskgate/taskgate/cmd"
	"github.com/ynny-github/taskgate/taskgate/internal/show"
)

func main() {
	if err := cmd.Execute(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		var showErr *show.ExitError
		if errors.As(err, &showErr) {
			os.Exit(showErr.Code)
		}
		fmt.Fprintln(os.Stderr, "taskgate:", err)
		os.Exit(1)
	}
}
