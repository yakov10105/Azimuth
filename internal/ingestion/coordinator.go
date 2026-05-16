package ingestion

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
)

// CoordinatorConfig controls the ingestion run.
type CoordinatorConfig struct {
	// WalkConfig governs file discovery (extensions, ignore patterns).
	WalkConfig WalkConfig

	// Workers is the number of parallel parse goroutines. Defaults to 4.
	Workers int
}

// DefaultCoordinatorConfig returns a ready-to-use configuration with
// sensible defaults: no ignore patterns, 4 workers.
func DefaultCoordinatorConfig() CoordinatorConfig {
	return CoordinatorConfig{
		Workers: 4,
	}
}

// IngestionReport summarises the outcome of a single Run.
// Idempotency (de-duplicating records in Neo4j) is enforced at the graph-write
// layer (Epic 1.3) via MERGE rather than CREATE; the coordinator itself is
// stateless and produces the same report on repeated runs of the same repo.
type IngestionReport struct {
	FilesProcessed   int
	GoFilesFound     int
	CSharpFilesFound int
	SymbolsExtracted int // functions + methods + structs + interfaces + classes
	EdgesFound       int // resolved CallEdge entries
	Errors           []string
}

// Coordinator orchestrates the full ingestion pipeline for a repository:
// walk → parse (concurrent) → resolve → report.
type Coordinator struct {
	cfg      CoordinatorConfig
	goParser *GoParser
	csParser *CSharpParser
	resolver *CallResolver
}

// NewCoordinator wires up all pipeline components.
func NewCoordinator(cfg CoordinatorConfig) *Coordinator {
	workers := cfg.Workers
	if workers <= 0 {
		workers = 4
	}
	cfg.Workers = workers
	return &Coordinator{
		cfg:      cfg,
		goParser: NewGoParser(),
		csParser: NewCSharpParser(),
		resolver: NewCallResolver(),
	}
}

// parseResult carries the outcome of parsing a single file.
type parseResult struct {
	path   string
	goFile *GoFile
	csFile *CSharpFile
	err    error
}

// Run executes the ingestion pipeline against root and returns a report.
// A non-nil error is returned only for infrastructure failures (e.g. walk
// cannot open the root directory). Per-file parse errors are collected into
// report.Errors and never abort the run.
func (c *Coordinator) Run(ctx context.Context, root string) (*IngestionReport, error) {
	paths, err := Walk(root, c.cfg.WalkConfig)
	if err != nil {
		return nil, fmt.Errorf("coordinator: walk %s: %w", root, err)
	}

	results := c.parseAll(ctx, paths)

	var goFiles []*GoFile
	var csFiles []*CSharpFile
	report := &IngestionReport{}
	processed := 0

	for r := range results {
		ext := filepath.Ext(r.path)

		if r.err != nil {
			// ErrCGoRequired is logged once and not added to report.Errors.
			if errors.Is(r.err, ErrCGoRequired) {
				slog.Warn("ingestion: C# parsing unavailable (CGo not enabled); skipping .cs files")
				continue
			}
			report.Errors = append(report.Errors, fmt.Sprintf("parse %s: %s", r.path, r.err))
			slog.Warn("ingestion: parse error", "path", r.path, "err", r.err)
			continue
		}

		switch ext {
		case ".go":
			report.GoFilesFound++
			report.FilesProcessed++
			report.SymbolsExtracted += countGoSymbols(r.goFile)
			if r.goFile.HasErrors {
				report.Errors = append(report.Errors,
					fmt.Sprintf("go parse %s: syntax errors (partial results retained)", r.path))
			}
			goFiles = append(goFiles, r.goFile)

		case ".cs":
			report.CSharpFilesFound++
			report.FilesProcessed++
			report.SymbolsExtracted += countCSharpSymbols(r.csFile)
			csFiles = append(csFiles, r.csFile)
		}

		processed++
		if processed%progressInterval == 0 {
			slog.Info("ingestion progress",
				"files_processed", processed,
				"total_files", len(paths),
			)
		}
	}

	// Surface context cancellation — workers exit silently, so check here.
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("coordinator: context cancelled: %w", err)
	}

	// Resolve call graphs and accumulate edges.
	goEdges := c.resolver.ResolveGo(goFiles)
	report.EdgesFound += len(goEdges)

	csEdges := c.resolver.ResolveCSharp(csFiles)
	report.EdgesFound += len(csEdges)

	slog.Info("ingestion complete",
		"root", root,
		"files_processed", report.FilesProcessed,
		"symbols_extracted", report.SymbolsExtracted,
		"edges_found", report.EdgesFound,
		"errors", len(report.Errors),
	)

	return report, nil
}

// progressInterval controls how often a progress log line is emitted.
const progressInterval = 100

// parseAll fans out path parsing across cfg.Workers goroutines and returns a
// channel of results. The channel is closed when all workers finish or ctx is
// cancelled. Workers respect ctx cancellation between files.
func (c *Coordinator) parseAll(ctx context.Context, paths []string) <-chan parseResult {
	jobs := make(chan string, len(paths))
	out := make(chan parseResult, len(paths))

	var wg sync.WaitGroup
	for i := 0; i < c.cfg.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				if ctx.Err() != nil {
					return
				}
				out <- c.parseOne(path)
			}
		}()
	}

	for _, p := range paths {
		jobs <- p
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}

// parseOne parses a single file and returns a parseResult.
func (c *Coordinator) parseOne(path string) parseResult {
	switch filepath.Ext(path) {
	case ".go":
		gf, err := c.goParser.ParseFile(path)
		return parseResult{path: path, goFile: gf, err: err}
	case ".cs":
		cf, err := c.csParser.ParseFile(path)
		return parseResult{path: path, csFile: cf, err: err}
	default:
		return parseResult{path: path}
	}
}

// countGoSymbols returns the total symbol count for a parsed Go file.
func countGoSymbols(f *GoFile) int {
	return len(f.Functions) + len(f.Methods) + len(f.Structs) + len(f.Interfaces)
}

// countCSharpSymbols returns the total symbol count for a parsed C# file.
func countCSharpSymbols(f *CSharpFile) int {
	count := len(f.Interfaces)
	for _, cls := range f.Classes {
		count++ // the class itself
		count += len(cls.Methods)
	}
	return count
}
