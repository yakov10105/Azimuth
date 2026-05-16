//go:build integration

package graph

import (
	"context"
	"testing"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupQueryTest seeds a known subgraph and returns a ready Neo4jQuery.
// Graph topology (prefix = "itest_q_"):
//
//	func2 ─CALLS→ func1 ←CALLS─ func3
//	func4 ─CALLS→ func2
//	MyStruct ─IMPLEMENTS→ MyIface
func setupQueryTest(t *testing.T) (neo4j.DriverWithContext, *Neo4jQuery) {
	t.Helper()
	uri := env("NEO4J_URI", "bolt://localhost:7687")
	user := env("NEO4J_USER", "neo4j")
	pass := env("NEO4J_PASSWORD", "")
	if pass == "" {
		t.Skip("NEO4J_PASSWORD not set — skipping integration test")
	}
	driver, err := neo4j.NewDriverWithContext(uri, neo4j.BasicAuth(user, pass, ""))
	require.NoError(t, err)
	require.NoError(t, driver.VerifyConnectivity(context.Background()))
	t.Cleanup(func() {
		deleteByPrefix(t, driver, "itest_q_")
		driver.Close(context.Background())
	})

	w := NewWriter(driver, DefaultWriterConfig())
	batch := WriteBatch{
		Functions: []NodeFunction{
			{FQN: "itest_q_func1", Name: "func1", FilePath: "/q.go", Language: "go"},
			{FQN: "itest_q_func2", Name: "func2", FilePath: "/q.go", Language: "go"},
			{FQN: "itest_q_func3", Name: "func3", FilePath: "/q.go", Language: "go"},
			{FQN: "itest_q_func4", Name: "func4", FilePath: "/q.go", Language: "go"},
		},
		Structs: []NodeStruct{
			{FQN: "itest_q_MyStruct", Name: "MyStruct", Package: "pkg", FilePath: "/q.go"},
		},
		Interfaces: []NodeInterface{
			{FQN: "itest_q_MyIface", Name: "MyIface", PackageOrNamespace: "pkg", FilePath: "/q.go"},
		},
		Calls: []EdgeCalls{
			{CallerFQN: "itest_q_func2", CalleeFQN: "itest_q_func1", CallSiteFile: "/q.go", CallSiteLine: 10},
			{CallerFQN: "itest_q_func3", CalleeFQN: "itest_q_func1", CallSiteFile: "/q.go", CallSiteLine: 20},
			{CallerFQN: "itest_q_func4", CalleeFQN: "itest_q_func2", CallSiteFile: "/q.go", CallSiteLine: 30},
		},
		Implements: []EdgeImplements{
			{ImplementorFQN: "itest_q_MyStruct", InterfaceFQN: "itest_q_MyIface"},
		},
	}
	_, err = w.Write(context.Background(), batch)
	require.NoError(t, err)

	return driver, NewNeo4jQuery(driver)
}

func TestNeo4jQuery_Integration_FindByName(t *testing.T) {
	_, q := setupQueryTest(t)
	nodes, err := q.FindByName(context.Background(), "func1")
	require.NoError(t, err)
	require.NotEmpty(t, nodes)
	found := false
	for _, n := range nodes {
		if fqn, _ := n.Properties["fqn"].(string); fqn == "itest_q_func1" {
			found = true
		}
	}
	assert.True(t, found, "func1 must appear in FindByName results")
}

func TestNeo4jQuery_Integration_FindCallers(t *testing.T) {
	_, q := setupQueryTest(t)
	// func1 is called by func2 and func3 directly; func4 calls func2→func1 (depth 2)
	edges, err := q.FindCallers(context.Background(), "itest_q_func1", 1)
	require.NoError(t, err)
	callers := make(map[string]bool)
	for _, e := range edges {
		callers[e.StartFQN] = true
	}
	assert.True(t, callers["itest_q_func2"], "func2 must be a direct caller")
	assert.True(t, callers["itest_q_func3"], "func3 must be a direct caller")

	// At depth 2, func4 should also appear (via func2)
	edges2, err := q.FindCallers(context.Background(), "itest_q_func1", 2)
	require.NoError(t, err)
	callers2 := make(map[string]bool)
	for _, e := range edges2 {
		callers2[e.StartFQN] = true
	}
	assert.True(t, callers2["itest_q_func4"], "func4 must appear as depth-2 caller")
}

func TestNeo4jQuery_Integration_FindCallees(t *testing.T) {
	_, q := setupQueryTest(t)
	edges, err := q.FindCallees(context.Background(), "itest_q_func4", 2)
	require.NoError(t, err)
	callees := make(map[string]bool)
	for _, e := range edges {
		callees[e.EndFQN] = true
	}
	assert.True(t, callees["itest_q_func2"], "func2 is a direct callee of func4")
	assert.True(t, callees["itest_q_func1"], "func1 is a depth-2 callee of func4")
}

func TestNeo4jQuery_Integration_FindImplementors(t *testing.T) {
	_, q := setupQueryTest(t)
	nodes, err := q.FindImplementors(context.Background(), "itest_q_MyIface")
	require.NoError(t, err)
	require.NotEmpty(t, nodes)
	found := false
	for _, n := range nodes {
		if fqn, _ := n.Properties["fqn"].(string); fqn == "itest_q_MyStruct" {
			found = true
		}
	}
	assert.True(t, found, "MyStruct must appear as implementor")
}

func TestNeo4jQuery_Integration_FindEntryPoints(t *testing.T) {
	_, q := setupQueryTest(t)
	nodes, err := q.FindEntryPoints(context.Background(), []string{"func1", "func2"})
	require.NoError(t, err)
	assert.NotEmpty(t, nodes)
}

func TestNeo4jQuery_Integration_FindEntryPoints_EmptyKeywords(t *testing.T) {
	_, q := setupQueryTest(t)
	nodes, err := q.FindEntryPoints(context.Background(), []string{})
	require.NoError(t, err)
	assert.Empty(t, nodes)
}

func TestNeo4jQuery_Integration_GetSubgraph(t *testing.T) {
	_, q := setupQueryTest(t)
	sg, err := q.GetSubgraph(context.Background(), "itest_q_func4", 2)
	require.NoError(t, err)
	require.NotNil(t, sg)

	fqns := make(map[string]bool)
	for _, n := range sg.Nodes {
		if fqn, _ := n.Properties["fqn"].(string); fqn != "" {
			fqns[fqn] = true
		}
	}
	assert.True(t, fqns["itest_q_func4"], "root must be present")
	assert.True(t, fqns["itest_q_func2"], "direct callee must be present")
	assert.True(t, fqns["itest_q_func1"], "depth-2 callee must be present")
	assert.NotEmpty(t, sg.Edges)
}

// TestNeo4jQuery_Integration_Performance asserts that all six query methods
// complete within 200 ms on the small seed graph (not a load test — just a
// regression guard to catch accidentally unbounded queries).
func TestNeo4jQuery_Integration_Performance(t *testing.T) {
	_, q := setupQueryTest(t)
	ctx := context.Background()
	const limit = 200 * time.Millisecond

	cases := []struct {
		name string
		fn   func() error
	}{
		{"FindByName", func() error { _, err := q.FindByName(ctx, "func"); return err }},
		{"FindCallers", func() error { _, err := q.FindCallers(ctx, "itest_q_func1", 3); return err }},
		{"FindCallees", func() error { _, err := q.FindCallees(ctx, "itest_q_func4", 3); return err }},
		{"FindImplementors", func() error { _, err := q.FindImplementors(ctx, "itest_q_MyIface"); return err }},
		{"FindEntryPoints", func() error { _, err := q.FindEntryPoints(ctx, []string{"func"}); return err }},
		{"GetSubgraph", func() error { _, err := q.GetSubgraph(ctx, "itest_q_func4", 3); return err }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			require.NoError(t, tc.fn())
			elapsed := time.Since(start)
			assert.Less(t, elapsed, limit, "%s must complete in < %s; took %s", tc.name, limit, elapsed)
		})
	}
}
