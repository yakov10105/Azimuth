//go:build integration

package graph

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupWriterTest connects to Neo4j and returns a ready Writer.
// Skips if NEO4J_PASSWORD is unset.
func setupWriterTest(t *testing.T) (neo4j.DriverWithContext, *Writer) {
	t.Helper()
	uri := env("NEO4J_URI", "bolt://localhost:7687")
	user := env("NEO4J_USER", "neo4j")
	pass := env("NEO4J_PASSWORD", "")
	if pass == "" {
		t.Skip("NEO4J_PASSWORD not set — skipping integration test")
	}
	driver, err := neo4j.NewDriverWithContext(uri, neo4j.BasicAuth(user, pass, ""))
	require.NoError(t, err)
	require.NoError(t, driver.VerifyConnectivity(context.Background()),
		"Neo4j unreachable at %s", uri)
	t.Cleanup(func() { driver.Close(context.Background()) })
	cfg := DefaultWriterConfig()
	cfg.Workers = 4
	return driver, NewWriter(driver, cfg)
}

// deleteByPrefix removes all nodes whose fqn or path starts with prefix.
func deleteByPrefix(t *testing.T, driver neo4j.DriverWithContext, prefix string) {
	t.Helper()
	session := driver.NewSession(context.Background(), neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(context.Background())
	result, err := session.Run(context.Background(),
		`MATCH (n) WHERE (n.fqn STARTS WITH $p OR n.path STARTS WITH $p) DETACH DELETE n`,
		map[string]any{"p": prefix})
	require.NoError(t, err)
	_, err = result.Consume(context.Background())
	require.NoError(t, err)
}

// countByLabel returns the number of nodes with the given label whose fqn starts with prefix.
func countByLabel(t *testing.T, driver neo4j.DriverWithContext, label, fqnPrefix string) int64 {
	t.Helper()
	session := driver.NewSession(context.Background(), neo4j.SessionConfig{})
	defer session.Close(context.Background())
	cypher := fmt.Sprintf(`MATCH (n:%s) WHERE n.fqn STARTS WITH $p RETURN count(n) AS cnt`, label)
	result, err := session.Run(context.Background(), cypher, map[string]any{"p": fqnPrefix})
	require.NoError(t, err)
	record, err := result.Single(context.Background())
	require.NoError(t, err)
	return record.Values[0].(int64)
}

func TestWriter_Integration_InsertNewFunction(t *testing.T) {
	driver, w := setupWriterTest(t)
	const prefix = "itest_insert_"
	t.Cleanup(func() { deleteByPrefix(t, driver, prefix) })

	fn := NodeFunction{
		FQN: prefix + "pkg.Foo", Name: "Foo",
		FilePath: "/repo/main.go", StartLine: 10, EndLine: 20, Language: "go",
	}
	report, err := w.Write(context.Background(), WriteBatch{Functions: []NodeFunction{fn}})
	require.NoError(t, err)
	assert.Equal(t, 1, report.NodesWritten)
	assert.Equal(t, int64(1), countByLabel(t, driver, "Function", prefix))
}

func TestWriter_Integration_UpdateExistingFunction(t *testing.T) {
	driver, w := setupWriterTest(t)
	const prefix = "itest_update_"
	t.Cleanup(func() { deleteByPrefix(t, driver, prefix) })

	fn := NodeFunction{FQN: prefix + "pkg.Bar", Name: "Bar", FilePath: "/f.go", Language: "go"}
	_, err := w.Write(context.Background(), WriteBatch{Functions: []NodeFunction{fn}})
	require.NoError(t, err)

	// Update the name via a second write with the same FQN — MERGE must not duplicate.
	fn.Name = "BarUpdated"
	_, err = w.Write(context.Background(), WriteBatch{Functions: []NodeFunction{fn}})
	require.NoError(t, err)

	assert.Equal(t, int64(1), countByLabel(t, driver, "Function", prefix), "MERGE must not duplicate")

	session := driver.NewSession(context.Background(), neo4j.SessionConfig{})
	defer session.Close(context.Background())
	result, err := session.Run(context.Background(),
		`MATCH (f:Function {fqn: $fqn}) RETURN f.name`,
		map[string]any{"fqn": fn.FQN})
	require.NoError(t, err)
	record, err := result.Single(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "BarUpdated", record.Values[0].(string))
}

func TestWriter_Integration_NoDuplicateOnMerge(t *testing.T) {
	driver, w := setupWriterTest(t)
	const prefix = "itest_dedup_"
	t.Cleanup(func() { deleteByPrefix(t, driver, prefix) })

	fn := NodeFunction{FQN: prefix + "pkg.Baz", Name: "Baz", FilePath: "/f.go", Language: "go"}
	for i := 0; i < 5; i++ {
		_, err := w.Write(context.Background(), WriteBatch{Functions: []NodeFunction{fn}})
		require.NoError(t, err)
	}
	assert.Equal(t, int64(1), countByLabel(t, driver, "Function", prefix),
		"writing the same FQN 5 times must produce exactly 1 node")
}

func TestWriter_Integration_InsertCallsEdge(t *testing.T) {
	driver, w := setupWriterTest(t)
	const prefix = "itest_calls_"
	t.Cleanup(func() { deleteByPrefix(t, driver, prefix) })

	caller := NodeFunction{FQN: prefix + "pkg.Caller", Name: "Caller", FilePath: "/f.go", Language: "go"}
	callee := NodeFunction{FQN: prefix + "pkg.Callee", Name: "Callee", FilePath: "/f.go", Language: "go"}
	edge := EdgeCalls{
		CallerFQN: caller.FQN, CalleeFQN: callee.FQN,
		CallSiteFile: "/f.go", CallSiteLine: 5,
	}
	_, err := w.Write(context.Background(), WriteBatch{
		Functions: []NodeFunction{caller, callee},
		Calls:     []EdgeCalls{edge},
	})
	require.NoError(t, err)

	session := driver.NewSession(context.Background(), neo4j.SessionConfig{})
	defer session.Close(context.Background())
	result, err := session.Run(context.Background(),
		`MATCH (a:Function {fqn: $caller})-[r:CALLS]->(b:Function {fqn: $callee})
		 RETURN count(r) AS cnt`,
		map[string]any{"caller": caller.FQN, "callee": callee.FQN})
	require.NoError(t, err)
	record, err := result.Single(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(1), record.Values[0].(int64), "exactly one CALLS edge must exist")
}

// TestWriter_Integration_ScaleTest writes 10,000 Function nodes and 50,000 CALLS edges
// and asserts the operation completes within 30 seconds with the correct counts in Neo4j.
func TestWriter_Integration_ScaleTest(t *testing.T) {
	driver, w := setupWriterTest(t)
	const (
		prefix = "itest_scale_"
		n      = 10_000
	)
	t.Cleanup(func() { deleteByPrefix(t, driver, prefix) })

	functions := make([]NodeFunction, n)
	for i := range functions {
		functions[i] = NodeFunction{
			FQN:      fmt.Sprintf("%sfunc%d", prefix, i),
			Name:     fmt.Sprintf("func%d", i),
			FilePath: "/scale/main.go",
			Language: "go",
		}
	}

	// Each function calls the next 5 functions (wrapping around) → 50,000 edges.
	calls := make([]EdgeCalls, 0, n*5)
	for i := 0; i < n; i++ {
		for j := 1; j <= 5; j++ {
			calls = append(calls, EdgeCalls{
				CallerFQN:    fmt.Sprintf("%sfunc%d", prefix, i),
				CalleeFQN:    fmt.Sprintf("%sfunc%d", prefix, (i+j)%n),
				CallSiteFile: "/scale/main.go",
				CallSiteLine: i*10 + j,
			})
		}
	}

	start := time.Now()
	report, err := w.Write(context.Background(), WriteBatch{Functions: functions, Calls: calls})
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Equal(t, n, report.NodesWritten)
	assert.Equal(t, n*5, report.EdgesWritten)
	assert.Less(t, elapsed, 30*time.Second,
		"scale write must complete in < 30s; took %s", elapsed)

	assert.Equal(t, int64(n), countByLabel(t, driver, "Function", prefix),
		"Neo4j must contain exactly %d Function nodes", n)
}
