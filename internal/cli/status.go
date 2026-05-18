package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewStatusCmd returns the `zm status` command.
func NewStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show knowledge graph index health",
		Long:  "Print node count, edge count, and last ingestion time from the connected Neo4j instance.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Full implementation in Task 3.6.2.
			fmt.Fprintln(cmd.OutOrStdout(), "status: not yet implemented")
			return nil
		},
	}
}
