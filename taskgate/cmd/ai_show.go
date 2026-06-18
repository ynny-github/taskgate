package cmd

import (
	"github.com/spf13/cobra"

	"github.com/ynny-github/taskgate/taskgate/internal/show"
)

func newAIShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "show [name]",
		Short:         "Show tasks and directories with summaries (merged shared+ai view), in structured form",
		Args:          cobra.MaximumNArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			code, err := show.Run(show.AudienceAI, args, cmd.OutOrStdout(), cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			if code != show.ExitSuccess {
				return &show.ExitError{Code: code}
			}
			return nil
		},
	}
}
