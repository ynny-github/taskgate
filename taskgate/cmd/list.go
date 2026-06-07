// taskgate/cmd/list.go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "list",
		Short:         "List tasks available to 'taskgate run'",
		Args:          cobra.NoArgs,
		RunE:          runList,
		SilenceErrors: true,
		SilenceUsage:  true,
	}
}

func runList(cmd *cobra.Command, _ []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot determine working directory: %w", err)
	}

	taskgateDir := filepath.Join(cwd, ".taskgate")
	if _, err := os.Stat(taskgateDir); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf(".taskgate/ not found")
		}
		return fmt.Errorf("cannot access .taskgate/: %w", err)
	}

	for _, subdir := range []string{"human", "shared"} {
		names, err := listScripts(filepath.Join(taskgateDir, subdir))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("cannot read .taskgate/%s/: %w", subdir, err)
		}
		sort.Strings(names)
		for _, name := range names {
			fmt.Fprintf(cmd.OutOrStdout(), "%s/%s\n", subdir, name)
		}
	}
	return nil
}
