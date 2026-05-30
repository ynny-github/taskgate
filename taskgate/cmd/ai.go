// taskgate/cmd/ai.go
package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// snapshotDirOverride is set in tests to bypass git root detection.
var snapshotDirOverride func(cwd string) (string, error)

func snapshotDirFn(cwd string) (string, error) {
	root := detectProjectRoot(cwd)
	if root == "" {
		return "", fmt.Errorf("cannot determine project root: not in a git repository")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".taskgate", "snapshots", filepath.Base(root)), nil
}

func newAICmd() *cobra.Command {
	aiCmd := &cobra.Command{
		Use:          "ai",
		Short:        "AI-facing taskgate commands",
		SilenceUsage: true,
	}
	aiCmd.AddCommand(newAIRunCmd())
	return aiCmd
}

func newAIRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "run <task-name> [-- args...]",
		Short:         "Run an AI task from the snapshot directory",
		Args:          cobra.MinimumNArgs(1),
		RunE:          runAITask,
		SilenceErrors: true,
	}
}

func runAITask(cmd *cobra.Command, args []string) error {
	taskName := args[0]
	scriptArgs := args[1:]

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot determine working directory: %w", err)
	}

	dirFn := snapshotDirFn
	if snapshotDirOverride != nil {
		dirFn = snapshotDirOverride
	}
	dir, err := dirFn(cwd)
	if err != nil {
		return err
	}

	taskPath, err := resolveAITask(dir, taskName)
	if err != nil {
		return err
	}

	if err := checkSnapshotFresh(cwd, taskName, taskPath); err != nil {
		fmt.Fprintln(cmd.ErrOrStderr(), err.Error())
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

func checkSnapshotFresh(cwd, taskName, snapshotPath string) error {
	var sourcePath string
	for _, subdir := range []string{"ai", "shared"} {
		p := filepath.Join(cwd, ".taskgate", subdir, taskName)
		if _, err := os.Stat(p); err == nil {
			sourcePath = p
			break
		}
	}
	if sourcePath == "" {
		return nil
	}

	snapshotBytes, err := os.ReadFile(snapshotPath)
	if err != nil {
		return fmt.Errorf("cannot read snapshot for %q: %w", taskName, err)
	}
	sourceBytes, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("cannot read source for %q: %w", taskName, err)
	}

	if !bytes.Equal(snapshotBytes, sourceBytes) {
		return fmt.Errorf("snapshot for %q is out of date; ask a human to run 'taskgate snapshot install' to review and approve the changes", taskName)
	}
	return nil
}

func resolveAITask(snapshotDir, taskName string) (string, error) {
	path := filepath.Join(snapshotDir, taskName)
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("task %q not found in snapshot dir (%s)", taskName, snapshotDir)
	}
	if err != nil {
		return "", fmt.Errorf("cannot access task %q: %w", taskName, err)
	}
	if info.Mode()&0111 == 0 {
		return "", fmt.Errorf("task %q is not executable", taskName)
	}
	return path, nil
}
