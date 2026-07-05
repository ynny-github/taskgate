// taskgate/cmd/init.go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "init",
		Short:         "Create .taskgate/ scaffold in the current directory",
		Args:          cobra.NoArgs,
		RunE:          runInit,
		SilenceErrors: true,
	}
}

func runInit(cmd *cobra.Command, _ []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot determine working directory: %w", err)
	}

	if err := scaffoldTaskgateDir(cmd, cwd); err != nil {
		return err
	}

	rel, action, err := ensureClaudeMdPointer(cwd)
	if err != nil {
		return fmt.Errorf("cannot update %s: %w", rel, err)
	}
	switch action {
	case "created":
		fmt.Fprintf(cmd.OutOrStdout(), "created %s with taskgate usage pointer\n", rel)
	case "updated":
		fmt.Fprintf(cmd.OutOrStdout(), "updated taskgate usage pointer in %s\n", rel)
	case "unchanged":
		fmt.Fprintf(cmd.OutOrStdout(), "taskgate usage pointer already current in %s\n", rel)
	}
	return nil
}

// scaffoldTaskgateDir creates .taskgate/{ai,human,shared} with example scripts.
// When .taskgate already exists it reports and skips creation.
func scaffoldTaskgateDir(cmd *cobra.Command, cwd string) error {
	taskgateDir := filepath.Join(cwd, ".taskgate")
	if _, err := os.Stat(taskgateDir); err == nil {
		fmt.Fprintf(cmd.ErrOrStderr(), ".taskgate/ already exists\n")
		return nil
	}

	for _, subdir := range []string{"ai", "human", "shared"} {
		if err := os.MkdirAll(filepath.Join(taskgateDir, subdir), 0755); err != nil {
			return fmt.Errorf("cannot create .taskgate/%s/: %w", subdir, err)
		}
		example := filepath.Join(taskgateDir, subdir, "example")
		content := fmt.Sprintf("#!/bin/sh\n# %s task — delete or replace this file\necho \"hello from %s/example\"\n", subdir, subdir)
		if err := os.WriteFile(example, []byte(content), 0755); err != nil {
			return fmt.Errorf("cannot create .taskgate/%s/example: %w", subdir, err)
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "created .taskgate/ with ai/, human/, shared/ subdirectories\n")
	return nil
}
