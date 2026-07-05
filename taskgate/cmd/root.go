// taskgate/cmd/root.go
package cmd

import "github.com/spf13/cobra"

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:          "taskgate",
		Short:        "Task runner — executes scripts from .taskgate/human/ and .taskgate/shared/",
		SilenceUsage: true,
	}
	root.AddCommand(newRunCmd())
	root.AddCommand(newAICmd())
	root.AddCommand(newSnapshotCmd())
	root.AddCommand(newInitCmd())
	root.AddCommand(newShowCmd())
	root.AddCommand(newValidateCmd())
	return root
}
