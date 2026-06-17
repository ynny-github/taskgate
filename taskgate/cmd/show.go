package cmd

import (
	"github.com/spf13/cobra"

	"github.com/ynny-github/taskgate/taskgate/internal/show"
)

func newShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "show [name]",
		Short:         "Show tasks and directories with summaries (merged shared+human view)",
		Args:          cobra.MaximumNArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			code, err := show.Run(show.AudienceHuman, args, cmd.OutOrStdout(), cmd.ErrOrStderr())
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
