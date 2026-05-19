package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/azimuth/azimuth/internal/orchestrator"
)

const defaultOrchestratorURL = "http://localhost:8000"

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
			return runAsk(cmd.Context(), cmd, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.JSON, "json", false, "Output raw JSON instead of Markdown")
	cmd.Flags().BoolVar(&opts.Verbose, "verbose", false, "Print intermediate agent outputs")
	cmd.Flags().BoolVar(&opts.Diagram, "diagram", false, "Generate a Mermaid diagram file alongside the answer")
	cmd.Flags().BoolVar(&opts.Full, "full", false, "Bypass answer truncation (may be slow)")
	cmd.Flags().IntVar(&opts.Depth, "depth", 3, "Graph traversal depth")

	return cmd
}

func runAsk(ctx context.Context, cmd *cobra.Command, opts AskOptions) error {
	orchestratorURL := os.Getenv("ORCHESTRATOR_URL")
	if orchestratorURL == "" {
		orchestratorURL = defaultOrchestratorURL
	}

	client := orchestrator.NewClient(orchestratorURL)

	if err := client.Ping(ctx); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: orchestrator service not running — start with: make orchestrator-up\n")
		os.Exit(2)
	}

	resp, err := client.Ask(ctx, orchestrator.AskRequest{
		Question: opts.Question,
		Depth:    opts.Depth,
	})
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: orchestrator request failed: %v\n", err)
		os.Exit(2)
	}

	if opts.JSON {
		return printAskJSON(cmd, resp)
	}
	printAskMarkdown(cmd, opts, resp)
	return nil
}

func printAskMarkdown(cmd *cobra.Command, opts AskOptions, resp *orchestrator.AskResponse) {
	out := cmd.OutOrStdout()

	if opts.Verbose {
		fmt.Fprintf(out, "Entry points found: %d\n", resp.EntryPointCount)
		fmt.Fprintf(out, "Graph bubble size: %d nodes\n\n", resp.GraphNodeCount)
	}

	fmt.Fprintf(out, "## %s\n\n", strings.TrimSpace(opts.Question))
	fmt.Fprintf(out, "%s\n", strings.TrimSpace(resp.Summary))

	if len(resp.CallPath) > 0 {
		fmt.Fprintf(out, "\n**Call path:**\n")
		for _, entry := range resp.CallPath {
			fmt.Fprintf(out, "- `%s`\n", entry)
		}
	}

	if len(resp.RelevantFiles) > 0 {
		fmt.Fprintf(out, "\n**Relevant files:**\n")
		for _, f := range resp.RelevantFiles {
			fmt.Fprintf(out, "- `%s`\n", f)
		}
	}
}

func printAskJSON(cmd *cobra.Command, resp *orchestrator.AskResponse) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	if err := enc.Encode(resp); err != nil {
		return fmt.Errorf("ask: encode JSON response: %w", err)
	}
	return nil
}
