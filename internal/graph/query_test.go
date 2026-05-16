package graph

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureExecutor records executeRead calls and returns configured rows/error.
type captureExecutor struct {
	calls  []captureCall
	rows   []queryRow
	retErr error
}

type captureCall struct {
	cypher string
	params map[string]any
}

func (c *captureExecutor) executeRead(_ context.Context, cypher string, params map[string]any) ([]queryRow, error) {
	c.calls = append(c.calls, captureCall{cypher, params})
	return c.rows, c.retErr
}

func newQueryWithCapture(cap *captureExecutor) *Neo4jQuery {
	return &Neo4jQuery{runner: cap}
}

// ─── FindByName ───────────────────────────────────────────────────────────────

func TestNeo4jQuery_FindByName_PropagatesError(t *testing.T) {
	cap := &captureExecutor{retErr: errors.New("neo4j down")}
	q := newQueryWithCapture(cap)
	_, err := q.FindByName(context.Background(), "Foo")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "FindByName")
}

func TestNeo4jQuery_FindByName_EmptyResult(t *testing.T) {
	cap := &captureExecutor{rows: []queryRow{}}
	q := newQueryWithCapture(cap)
	nodes, err := q.FindByName(context.Background(), "Foo")
	require.NoError(t, err)
	assert.NotNil(t, nodes)
	assert.Empty(t, nodes)
}

func TestNeo4jQuery_FindByName_PassesName(t *testing.T) {
	cap := &captureExecutor{}
	q := newQueryWithCapture(cap)
	_, _ = q.FindByName(context.Background(), "MyService")
	require.Len(t, cap.calls, 1)
	assert.Equal(t, "MyService", cap.calls[0].params["name"])
}

// ─── FindCallers ──────────────────────────────────────────────────────────────

func TestNeo4jQuery_FindCallers_ClampsDepthLow(t *testing.T) {
	cap := &captureExecutor{}
	q := newQueryWithCapture(cap)
	_, _ = q.FindCallers(context.Background(), "pkg.Foo", 0)
	require.Len(t, cap.calls, 1)
	assert.Equal(t, 1, cap.calls[0].params["depth"])
}

func TestNeo4jQuery_FindCallers_ClampsDepthHigh(t *testing.T) {
	cap := &captureExecutor{}
	q := newQueryWithCapture(cap)
	_, _ = q.FindCallers(context.Background(), "pkg.Foo", 10)
	require.Len(t, cap.calls, 1)
	assert.Equal(t, 5, cap.calls[0].params["depth"])
}

func TestNeo4jQuery_FindCallers_PropagatesError(t *testing.T) {
	cap := &captureExecutor{retErr: errors.New("timeout")}
	q := newQueryWithCapture(cap)
	_, err := q.FindCallers(context.Background(), "pkg.Foo", 2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "FindCallers")
}

// ─── FindCallees ──────────────────────────────────────────────────────────────

func TestNeo4jQuery_FindCallees_PropagatesError(t *testing.T) {
	cap := &captureExecutor{retErr: errors.New("timeout")}
	q := newQueryWithCapture(cap)
	_, err := q.FindCallees(context.Background(), "pkg.Foo", 2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "FindCallees")
}

// ─── FindImplementors ─────────────────────────────────────────────────────────

func TestNeo4jQuery_FindImplementors_PropagatesError(t *testing.T) {
	cap := &captureExecutor{retErr: errors.New("timeout")}
	q := newQueryWithCapture(cap)
	_, err := q.FindImplementors(context.Background(), "pkg.IFoo")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "FindImplementors")
}

// ─── FindEntryPoints ──────────────────────────────────────────────────────────

func TestNeo4jQuery_FindEntryPoints_EmptyKeywords(t *testing.T) {
	cap := &captureExecutor{}
	q := newQueryWithCapture(cap)
	nodes, err := q.FindEntryPoints(context.Background(), []string{})
	require.NoError(t, err)
	assert.Empty(t, nodes)
	assert.Len(t, cap.calls, 0, "empty keywords must skip DB call")
}

func TestNeo4jQuery_FindEntryPoints_PropagatesError(t *testing.T) {
	cap := &captureExecutor{retErr: errors.New("timeout")}
	q := newQueryWithCapture(cap)
	_, err := q.FindEntryPoints(context.Background(), []string{"Handler"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "FindEntryPoints")
}

// ─── GetSubgraph ──────────────────────────────────────────────────────────────

func TestNeo4jQuery_GetSubgraph_ClampsDepth(t *testing.T) {
	cap := &captureExecutor{rows: []queryRow{}}
	q := newQueryWithCapture(cap)
	_, _ = q.GetSubgraph(context.Background(), "pkg.Foo", 99)
	// Two calls: root lookup + traversal
	require.Len(t, cap.calls, 2)
	assert.Equal(t, 5, cap.calls[1].params["depth"])
}

func TestNeo4jQuery_GetSubgraph_RootAlwaysIncluded(t *testing.T) {
	cap := &captureExecutor{rows: []queryRow{}}
	q := newQueryWithCapture(cap)
	sg, err := q.GetSubgraph(context.Background(), "pkg.Foo", 1)
	require.NoError(t, err)
	// Root not found (empty rows from first call) — subgraph still non-nil, just empty
	assert.NotNil(t, sg)
}
