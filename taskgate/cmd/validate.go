// taskgate/cmd/validate.go
package cmd

import (
	"github.com/spf13/cobra"

	"github.com/ynny-github/taskgate/taskgate/internal/show"
	"github.com/ynny-github/taskgate/taskgate/internal/validate"
)

func newValidateCmd() *cobra.Command {
	return newValidateCmdFor(show.AudienceHuman, "Validate task files under .taskgate/")
}

func newAIValidateCmd() *cobra.Command {
	return newValidateCmdFor(show.AudienceAI, "Validate task files under .taskgate/ (AI JSON output)")
}

func newValidateCmdFor(audience show.Audience, short string) *cobra.Command {
	return &cobra.Command{
		Use:           "validate [name]",
		Short:         short,
		Args:          cobra.MaximumNArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			code, err := validate.Run(audience, args, cmd.OutOrStdout(), cmd.ErrOrStderr())
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
