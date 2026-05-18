package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// IngestOptions holds parsed flags for the ingest command.
type IngestOptions struct {
	RepoPath string
	DryRun   bool
	Language string
}

// NewIngestCmd returns the `zm ingest <repo-path>` command.
func NewIngestCmd() *cobra.Command {
	var opts IngestOptions

	cmd := &cobra.Command{
		Use:   "ingest <repo-path>",
		Short: "Index a repository into the knowledge graph",
		Long:  "Parse all Go and C# source files under <repo-path> and write the resulting nodes and edges to Neo4j.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.RepoPath = args[0]
			// Full implementation in Task 1.4.2.
			fmt.Fprintln(cmd.OutOrStdout(), "ingest: not yet implemented")
			return nil
		},
	}

	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Parse and extract without writing to stores")
	cmd.Flags().StringVar(&opts.Language, "language", "", "Restrict ingestion to: go, csharp")

	return cmd
}
