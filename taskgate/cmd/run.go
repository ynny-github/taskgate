// taskgate/cmd/run.go
package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:                "run <task-name> [args...]",
		Short:              "Run a task from .taskgate/human/ or .taskgate/shared/",
		Args:               cobra.MinimumNArgs(1),
		RunE:               runTask,
		SilenceErrors:      true,
		DisableFlagParsing: true,
	}
}

func runTask(cmd *cobra.Command, args []string) error {
	taskName := args[0]
	scriptArgs := args[1:]

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot determine working directory: %w", err)
	}

	taskPath, err := resolveHumanTask(cwd, taskName)
	if err != nil {
		return err
	}

	c := exec.Command(taskPath, scriptArgs...)
	c.Stdout = cmd.OutOrStdout()
	c.Stderr = cmd.ErrOrStderr()
	c.Stdin = os.Stdin

	if root := detectProjectRoot(cwd); root != "" {
		env := make([]string, 0, len(os.Environ())+1)
		for _, e := range os.Environ() {
			if !strings.HasPrefix(e, "TASKGATE_PROJECT_ROOT=") {
				env = append(env, e)
			}
		}
		c.Env = append(env, "TASKGATE_PROJECT_ROOT="+root)
	}

	return c.Run()
}

func detectProjectRoot(cwd string) string {
	out, err := exec.Command("git", "-C", cwd, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func resolveHumanTask(cwd, taskName string) (string, error) {
	for _, subdir := range []string{"human", "shared"} {
		path := filepath.Join(cwd, ".taskgate", subdir, taskName)
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("cannot access task %q: %w", taskName, err)
		}
		if info.Mode()&0111 == 0 {
			return "", fmt.Errorf("task %q is not executable", taskName)
		}
		return path, nil
	}
	return "", fmt.Errorf("task %q not found in .taskgate/human/ or .taskgate/shared/", taskName)
}
