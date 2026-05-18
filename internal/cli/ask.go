package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// AskOptions holds parsed flags for the ask command.
type AskOptions struct {
	Question string
	JSON     bool
	Verbose  bool
	Diagram  bool
	Full     bool
	Depth    int
}

// NewAskCmd returns the `zm ask "<question>"` command.
func NewAskCmd() *cobra.Command {
	var opts AskOptions

	cmd := &cobra.Command{
		Use:   `ask "<question>"`,
		Short: "Query the knowledge graph in natural language",
		Long:  "Run a natural-language question through the LangGraph pipeline and print the answer.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Question = args[0]
			// Full implementation in Task 1.5.6.
			fmt.Fprintln(cmd.OutOrStdout(), "ask: not yet implemented")
			return nil
		},
	}

	cmd.Flags().BoolVar(&opts.JSON, "json", false, "Output raw JSON instead of Markdown")
	cmd.Flags().BoolVar(&opts.Verbose, "verbose", false, "Print intermediate agent outputs")
	cmd.Flags().BoolVar(&opts.Diagram, "diagram", false, "Generate a Mermaid diagram file alongside the answer")
	cmd.Flags().BoolVar(&opts.Full, "full", false, "Bypass answer truncation (may be slow)")
	cmd.Flags().IntVar(&opts.Depth, "depth", 3, "Graph traversal depth")

	return cmd
}
