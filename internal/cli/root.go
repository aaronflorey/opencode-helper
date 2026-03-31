package cli

import "github.com/spf13/cobra"

func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "opencode-helper",
		Short: "Reconstruct files from OpenCode session history",
	}

	root.AddCommand(NewRestoreCommand())

	return root
}
