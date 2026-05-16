//go:build integration

package ingestion

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCoordinator_Integration_RealGoRepo ingests the synthetic callgraph fixture
// (testdata/callgraph/) which contains three real .go files with functions,
// methods, an interface, a struct, and cross-function calls.
//
// This test proves the full pipeline — walk → concurrent parse → resolve →
// report — runs without panics and produces non-zero, meaningful output.
func TestCoordinator_Integration_RealGoRepo(t *testing.T) {
	root := "testdata/callgraph"

	c := NewCoordinator(DefaultCoordinatorConfig())
	report, err := c.Run(context.Background(), root)
	require.NoError(t, err)

	// All three .go files must be processed.
	assert.Equal(t, 3, report.FilesProcessed, "expected all 3 .go files to be processed")
	assert.Equal(t, 3, report.GoFilesFound)
	assert.Equal(t, 0, report.CSharpFilesFound)

	// Symbols: Caller, Callee, helper (functions) + EnglishGreeter.Greet (method)
	//          + EnglishGreeter (struct) + Greeter (interface) = 6
	assert.GreaterOrEqual(t, report.SymbolsExtracted, 6,
		"expected at least 6 symbols (3 funcs, 1 method, 1 struct, 1 interface)")

	// Edges: Caller→Callee, Caller→fmt.Println, Caller→EnglishGreeter.Greet,
	//        Callee→helper — at minimum 4 directed edges.
	assert.GreaterOrEqual(t, report.EdgesFound, 4,
		"expected at least 4 call edges")

	assert.Empty(t, report.Errors, "no parse errors expected for well-formed fixture files")
}

// TestCoordinator_Integration_Idempotent verifies that running the coordinator
// twice on the same root produces identical reports, confirming the pipeline is
// stateless (Neo4j-level idempotency via MERGE is handled in Epic 1.3).
func TestCoordinator_Integration_Idempotent(t *testing.T) {
	root := "testdata/callgraph"

	c := NewCoordinator(DefaultCoordinatorConfig())

	r1, err := c.Run(context.Background(), root)
	require.NoError(t, err)

	r2, err := c.Run(context.Background(), root)
	require.NoError(t, err)

	assert.Equal(t, r1.FilesProcessed, r2.FilesProcessed, "FilesProcessed must be stable across runs")
	assert.Equal(t, r1.SymbolsExtracted, r2.SymbolsExtracted, "SymbolsExtracted must be stable across runs")
	assert.Equal(t, r1.EdgesFound, r2.EdgesFound, "EdgesFound must be stable across runs")
	assert.Equal(t, len(r1.Errors), len(r2.Errors), "error count must be stable across runs")
}
