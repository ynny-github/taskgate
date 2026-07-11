// taskgate/cmd/run.go
package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/ynny-github/taskgate/taskgate/internal/taskgraph"
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

	res := humanResolver{root: root}
	g, err := taskgraph.Build(taskName, res)
	if err != nil {
		return err
	}

	env := taskEnv(root)
	runner := func(path string, a []string) (int, error) {
		c := exec.Command(path, a...)
		c.Stdout = cmd.OutOrStdout()
		c.Stderr = cmd.ErrOrStderr()
		c.Stdin = os.Stdin
		c.Env = env
		if err := c.Run(); err != nil {
			var ee *exec.ExitError
			if errors.As(err, &ee) {
				return ee.ExitCode(), nil
			}
			return 0, err
		}
		return 0, nil
	}
	if code := taskgraph.Execute(g, scriptArgs, runner); code != 0 {
		return &exitError{code: code}
	}
	return nil
}

// humanResolver resolves dependency names across .taskgate/human then
// .taskgate/shared, classifying failures for taskgraph.
type humanResolver struct{ root string }

func (r humanResolver) Resolve(name string) (string, error) {
	if r.root == "" {
		return "", &taskgraph.ResolveError{Name: name, Kind: taskgraph.ResolveUnknown,
			Detail: "not found in .taskgate/human/ or .taskgate/shared/"}
	}
	for _, subdir := range []string{"human", "shared"} {
		path := filepath.Join(r.root, ".taskgate", subdir, name)
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&0o111 == 0 {
			return "", &taskgraph.ResolveError{Name: name, Kind: taskgraph.ResolveNotExecutable,
				Detail: "is not executable"}
		}
		return path, nil
	}
	return "", &taskgraph.ResolveError{Name: name, Kind: taskgraph.ResolveUnknown,
		Detail: "not found in .taskgate/human/ or .taskgate/shared/"}
}

// taskEnv returns the child environment with TASKGATE_PROJECT_ROOT managed.
func taskEnv(root string) []string {
	env := make([]string, 0, len(os.Environ())+1)
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "TASKGATE_PROJECT_ROOT=") {
			env = append(env, e)
		}
	}
	if root != "" {
		env = append(env, "TASKGATE_PROJECT_ROOT="+root)
	}
	return env
}

// exitError carries a child task's exit code up to Run without printing an
// extra diagnostic line (the child already wrote its own output).
type exitError struct{ code int }

func (e *exitError) Error() string { return fmt.Sprintf("task exited with code %d", e.code) }

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
