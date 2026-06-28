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

	root := detectProjectRoot(cwd)
	taskPath, err := resolveHumanTask(root, taskName)
	if err != nil {
		return err
	}

	c := exec.Command(taskPath, scriptArgs...)
	c.Stdout = cmd.OutOrStdout()
	c.Stderr = cmd.ErrOrStderr()
	c.Stdin = os.Stdin

	// Always manage TASKGATE_PROJECT_ROOT: set it if a project root is found,
	// otherwise ensure it's not passed to the child process.
	env := make([]string, 0, len(os.Environ())+1)
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "TASKGATE_PROJECT_ROOT=") {
			env = append(env, e)
		}
	}
	if root != "" {
		env = append(env, "TASKGATE_PROJECT_ROOT="+root)
	}
	c.Env = env

	return c.Run()
}

func detectProjectRoot(cwd string) string {
	dir, err := filepath.Abs(cwd)
	if err != nil {
		return ""
	}
	for {
		marker := filepath.Join(dir, ".taskgate")
		if info, err := os.Stat(marker); err == nil && info.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func resolveHumanTask(root, taskName string) (string, error) {
	if root == "" {
		return "", fmt.Errorf("task %q not found in .taskgate/human/ or .taskgate/shared/", taskName)
	}
	for _, subdir := range []string{"human", "shared"} {
		path := filepath.Join(root, ".taskgate", subdir, taskName)
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
