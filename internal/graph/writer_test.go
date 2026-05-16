package graph

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockTxExecutor is a thread-safe txExecutor stub for unit tests.
type mockTxExecutor struct {
	mu    sync.Mutex
	calls []mockCall
	errOn int // 1-indexed call number to return an error on; 0 = never
}

type mockCall struct {
	cypher string
	params map[string]any
}

func (m *mockTxExecutor) executeTx(_ context.Context, cypher string, params map[string]any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, mockCall{cypher, params})
	if m.errOn > 0 && len(m.calls) == m.errOn {
		return errors.New("simulated tx error")
	}
	return nil
}

// snapshot returns a copy of all recorded calls (safe for concurrent access).
func (m *mockTxExecutor) snapshot() []mockCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]mockCall, len(m.calls))
	copy(out, m.calls)
	return out
}

func newTestWriter(mock *mockTxExecutor, batchSize, workers int) *Writer {
	return &Writer{cfg: WriterConfig{BatchSize: batchSize, Workers: workers}, tx: mock}
}

func TestWriter_Write_EmptyBatch(t *testing.T) {
	mock := &mockTxExecutor{}
	w := newTestWriter(mock, 500, 1)
	report, err := w.Write(context.Background(), WriteBatch{})
	require.NoError(t, err)
	assert.Len(t, mock.snapshot(), 0)
	assert.Equal(t, 0, report.NodesWritten)
	assert.Equal(t, 0, report.EdgesWritten)
}

func TestWriter_Write_SplitsBatches(t *testing.T) {
	// 1200 functions with batchSize=500 → ceil(1200/500) = 3 node-phase calls.
	mock := &mockTxExecutor{}
	w := newTestWriter(mock, 500, 1)
	_, err := w.Write(context.Background(), WriteBatch{Functions: make([]NodeFunction, 1200)})
	require.NoError(t, err)
	assert.Len(t, mock.snapshot(), 3)
}

func TestWriter_Write_NodesBeforeEdges(t *testing.T) {
	// With Workers=1, execution is sequential and ordering is deterministic.
	mock := &mockTxExecutor{}
	w := newTestWriter(mock, 500, 1)
	batch := WriteBatch{
		Functions: []NodeFunction{{FQN: "pkg.Foo"}},
		Calls:     []EdgeCalls{{CallerFQN: "pkg.Foo", CalleeFQN: "pkg.Bar"}},
	}
	_, err := w.Write(context.Background(), batch)
	require.NoError(t, err)
	calls := mock.snapshot()
	require.Len(t, calls, 2)
	assert.Contains(t, calls[0].cypher, "Function", "first call must be a node write")
	assert.Contains(t, calls[1].cypher, "CALLS", "second call must be an edge write")
}

func TestWriter_Write_SkipsExternalCallees(t *testing.T) {
	mock := &mockTxExecutor{}
	w := newTestWriter(mock, 500, 1)
	batch := WriteBatch{
		Calls: []EdgeCalls{
			{CallerFQN: "pkg.Foo", CalleeFQN: "EXTERNAL::fmt.Println"},
			{CallerFQN: "pkg.Foo", CalleeFQN: "EXTERNAL::os.Exit"},
		},
	}
	report, err := w.Write(context.Background(), batch)
	require.NoError(t, err)
	assert.Len(t, mock.snapshot(), 0, "EXTERNAL callees must produce no write jobs")
	assert.Equal(t, 2, report.EdgesWritten, "report counts input edges regardless of skip")
}

func TestWriter_Write_AllCyphersUseMerge(t *testing.T) {
	mock := &mockTxExecutor{}
	w := newTestWriter(mock, 500, 1)
	batch := WriteBatch{
		Files:      []NodeFile{{Path: "/a.go", Language: "go"}},
		Functions:  []NodeFunction{{FQN: "pkg.Foo", Name: "Foo", Language: "go"}},
		Methods:    []NodeMethod{{FQN: "pkg.T.M", Name: "M", Receiver: "T"}},
		Classes:    []NodeClass{{FQN: "ns.C", Name: "C", Namespace: "ns"}},
		Structs:    []NodeStruct{{FQN: "pkg.S", Name: "S", Package: "pkg"}},
		Interfaces: []NodeInterface{{FQN: "pkg.I", Name: "I", PackageOrNamespace: "pkg"}},
		DefinedIn: []EdgeDefinedIn{
			{NodeFQN: "pkg.Foo", NodeLabel: "Function", FilePath: "/a.go"},
		},
	}
	_, err := w.Write(context.Background(), batch)
	require.NoError(t, err)
	for _, c := range mock.snapshot() {
		assert.True(t, strings.Contains(c.cypher, "MERGE"),
			"every Cypher must use MERGE: %q", c.cypher)
	}
}

func TestWriter_Write_StopsOnNodeError(t *testing.T) {
	// 3 function batches (1200/500); error on call 2.
	// With Workers=1, exactly 2 calls are made before cancellation.
	// The edge phase must not start because node phase returned an error.
	mock := &mockTxExecutor{errOn: 2}
	w := newTestWriter(mock, 500, 1)
	batch := WriteBatch{
		Functions: make([]NodeFunction, 1200),
		Calls:     []EdgeCalls{{CallerFQN: "a", CalleeFQN: "b"}},
	}
	_, err := w.Write(context.Background(), batch)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nodes")
	assert.Equal(t, 2, len(mock.snapshot()))
}

func TestWriter_Write_ConcurrentWorkers(t *testing.T) {
	// 10 functions, batchSize=1 → 10 node-phase jobs; Workers=3 must run all 10.
	mock := &mockTxExecutor{}
	w := newTestWriter(mock, 1, 3)
	_, err := w.Write(context.Background(), WriteBatch{Functions: make([]NodeFunction, 10)})
	require.NoError(t, err)
	assert.Len(t, mock.snapshot(), 10)
}
