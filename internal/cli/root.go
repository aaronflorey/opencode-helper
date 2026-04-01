package cli

import "github.com/spf13/cobra"

func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "opencode-helper",
		Short: "Utilities for OpenCode local data",
	}

	root.AddCommand(NewRestoreCommand())
	root.AddCommand(NewToolUsageCommand())
	root.AddCommand(NewUsageCommand())

	return root
}
