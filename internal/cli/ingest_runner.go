package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/spf13/cobra"

	"github.com/azimuth/azimuth/internal/config"
	"github.com/azimuth/azimuth/internal/graph"
	"github.com/azimuth/azimuth/internal/ingestion"
)

// runIngest is the real implementation of `zm ingest <repo-path>`.
// Validation errors are returned as Go errors (Cobra prints them, exits 1).
// Infrastructure errors call os.Exit(2) directly after printing to stderr.
func runIngest(ctx context.Context, cmd *cobra.Command, opts IngestOptions) error {
	if err := validateIngestArgs(opts); err != nil {
		return err
	}

	cfg, err := config.LoadRaw("")
	if err != nil {
		return fmt.Errorf("ingest: load config: %w", err)
	}
	if cfg.Neo4j.Password == "" {
		return fmt.Errorf("ingest: NEO4J_PASSWORD is required")
	}

	driver, err := neo4j.NewDriverWithContext(
		cfg.Neo4j.URI,
		neo4j.BasicAuth(cfg.Neo4j.User, cfg.Neo4j.Password, ""),
	)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: cannot create Neo4j driver: %v\n", err)
		os.Exit(2)
	}
	defer driver.Close(ctx)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := driver.VerifyConnectivity(pingCtx); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: Neo4j unreachable at %s: %v\n", cfg.Neo4j.URI, err)
		os.Exit(2)
	}

	sm := graph.NewSchemaManager()
	if err := sm.Migrate(ctx, driver); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: Neo4j schema migration failed: %v\n", err)
		os.Exit(2)
	}

	start := time.Now()

	fmt.Fprintf(cmd.ErrOrStderr(), "[1/3] Discovering and parsing files...\n")

	coordCfg := ingestion.DefaultCoordinatorConfig()
	coordCfg.Workers = cfg.App.WorkerPoolSize
	switch opts.Language {
	case "go":
		coordCfg.WalkConfig.Extensions = []string{".go"}
	case "csharp":
		coordCfg.WalkConfig.Extensions = []string{".cs"}
	}

	coord := ingestion.NewCoordinator(coordCfg)
	data, err := coord.RunFull(ctx, opts.RepoPath)
	if err != nil {
		return fmt.Errorf("ingest: pipeline: %w", err)
	}

	parseElapsed := time.Since(start)
	fmt.Fprintf(cmd.ErrOrStderr(),
		"[2/3] Parsing complete: %d files, %d symbols, %d edges (%.1fs)\n",
		data.Report.FilesProcessed,
		data.Report.SymbolsExtracted,
		data.Report.EdgesFound,
		parseElapsed.Seconds(),
	)

	batch := buildWriteBatch(data)
	nodeCount := len(batch.Files) + len(batch.Functions) + len(batch.Methods) +
		len(batch.Classes) + len(batch.Structs) + len(batch.Interfaces)
	edgeCount := len(batch.Calls) + len(batch.DefinedIn) +
		len(batch.Implements) + len(batch.HasMethod)

	if opts.DryRun {
		printDryRunSummary(cmd, data, nodeCount, edgeCount, time.Since(start))
		return nil
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "[3/3] Writing to Neo4j...\n")

	writerCfg := graph.DefaultWriterConfig()
	writerCfg.BatchSize = cfg.App.Neo4jWriteBatchSize
	w := graph.NewWriter(driver, writerCfg)

	writeReport, err := w.Write(ctx, batch)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: Neo4j write failed: %v\n", err)
		os.Exit(2)
	}

	printSummary(cmd, data.Report, writeReport, time.Since(start))
	return nil
}

// validateIngestArgs checks the repo path and flag values before any pipeline work.
func validateIngestArgs(opts IngestOptions) error {
	info, err := os.Stat(opts.RepoPath)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("'%s' is not a valid directory", opts.RepoPath)
	}
	if _, err := os.Stat(filepath.Join(opts.RepoPath, ".git")); os.IsNotExist(err) {
		return fmt.Errorf("'%s' is not a git repository (no .git directory)", opts.RepoPath)
	}
	if opts.Language != "" && opts.Language != "go" && opts.Language != "csharp" {
		return fmt.Errorf("--language must be 'go' or 'csharp', got '%s'", opts.Language)
	}
	return nil
}

// printSummary writes the post-ingestion summary to stdout.
func printSummary(cmd *cobra.Command, report *ingestion.IngestionReport, wr *graph.WriteReport, elapsed time.Duration) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "\nIngestion complete in %.1fs\n", elapsed.Seconds())
	fmt.Fprintf(out, "  Files:  %d parsed, %d errors\n", report.FilesProcessed, len(report.Errors))
	fmt.Fprintf(out, "  Nodes:  %d written\n", wr.NodesWritten)
	fmt.Fprintf(out, "  Edges:  %d written\n", wr.EdgesWritten)
}

// printDryRunSummary writes a dry-run report to stdout.
func printDryRunSummary(cmd *cobra.Command, data *ingestion.PipelineData, nodes, edges int, elapsed time.Duration) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "\nIngestion complete (dry-run) in %.1fs\n", elapsed.Seconds())
	fmt.Fprintf(out, "  Files:  %d parsed, %d errors\n",
		data.Report.FilesProcessed, len(data.Report.Errors))
	fmt.Fprintf(out, "  Nodes:  %d (would write)\n", nodes)
	fmt.Fprintf(out, "  Edges:  %d (would write)\n", edges)
}
