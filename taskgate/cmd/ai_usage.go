package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ynny-github/taskgate/taskgate/internal/usage"
)

func newAIUsageCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "usage",
		Short: "Print the taskgate usage guide for AI agents (Markdown, not JSON)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprint(cmd.OutOrStdout(), usage.Guide())
			return err
		},
	}
}
