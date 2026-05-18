package cli

import "github.com/spf13/cobra"

// NewRootCmd returns the zm root command. It is thin — it only groups
// subcommands and exposes --version / --help.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     "zm",
		Short:   "Azimuth — codebase oracle",
		Long:    "Azimuth indexes Go and C# repositories into a knowledge graph and answers architectural questions.",
		Version: Version,
	}

	root.AddCommand(
		NewIngestCmd(),
		NewAskCmd(),
		NewStatusCmd(),
	)

	return root
}
