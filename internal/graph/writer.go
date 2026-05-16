package graph

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// txExecutor runs a single Cypher statement inside a write transaction.
// The real implementation uses neo4j.SessionWithContext.ExecuteWrite;
// tests inject a stub so no live Neo4j is required.
type txExecutor interface {
	executeTx(ctx context.Context, cypher string, params map[string]any) error
}

// neo4jTxExecutor is the production txExecutor backed by a real Neo4j driver.
type neo4jTxExecutor struct {
	driver neo4j.DriverWithContext
}

func (e *neo4jTxExecutor) executeTx(ctx context.Context, cypher string, params map[string]any) error {
	session := e.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)
	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, cypher, params)
		if err != nil {
			return nil, err
		}
		_, err = result.Consume(ctx)
		return nil, err
	})
	if err != nil {
		slog.Warn("neo4j batch write failed", "err", err)
		return fmt.Errorf("neo4j tx: %w", err)
	}
	return nil
}

// WriterConfig controls batching and concurrency for the write layer.
type WriterConfig struct {
	BatchSize int // nodes or edges per UNWIND transaction; default 500
	Workers   int // concurrent write goroutines; default 4
}

// DefaultWriterConfig returns sensible production defaults.
func DefaultWriterConfig() WriterConfig {
	return WriterConfig{BatchSize: 500, Workers: 4}
}

// ─── Node types ───────────────────────────────────────────────────────────────

// NodeFile represents a :File node.
type NodeFile struct {
	Path     string
	Language string
	Package  string
}

// NodeFunction represents a :Function node.
type NodeFunction struct {
	FQN       string
	Name      string
	FilePath  string
	StartLine int
	EndLine   int
	Language  string
}

// NodeMethod represents a :Method node.
type NodeMethod struct {
	FQN       string
	Name      string
	Receiver  string
	FilePath  string
	StartLine int
	EndLine   int
}

// NodeClass represents a :Class node.
type NodeClass struct {
	FQN       string
	Name      string
	Namespace string
	FilePath  string
}

// NodeStruct represents a :Struct node.
type NodeStruct struct {
	FQN      string
	Name     string
	Package  string
	FilePath string
}

// NodeInterface represents an :Interface node.
type NodeInterface struct {
	FQN                string
	Name               string
	PackageOrNamespace string
	FilePath           string
}

// ─── Edge types ───────────────────────────────────────────────────────────────

// EdgeCalls represents a [:CALLS] relationship.
// Edges whose CalleeFQN begins with "EXTERNAL::" are silently skipped —
// those callees have no corresponding Neo4j node.
type EdgeCalls struct {
	CallerFQN    string
	CalleeFQN    string
	CallSiteFile string
	CallSiteLine int
}

// EdgeDefinedIn represents a [:DEFINED_IN] relationship.
// NodeLabel must be one of: Function, Method, Class, Struct, Interface.
type EdgeDefinedIn struct {
	NodeFQN   string
	NodeLabel string
	FilePath  string
}

// EdgeImplements represents an [:IMPLEMENTS] relationship from a Class or Struct to an Interface.
type EdgeImplements struct {
	ImplementorFQN string
	InterfaceFQN   string
}

// EdgeHasMethod represents a [:HAS_METHOD] relationship from a Class or Struct to a Method.
type EdgeHasMethod struct {
	OwnerFQN  string
	MethodFQN string
}

// WriteBatch groups all nodes and edges for a single Write call.
type WriteBatch struct {
	Files      []NodeFile
	Functions  []NodeFunction
	Methods    []NodeMethod
	Classes    []NodeClass
	Structs    []NodeStruct
	Interfaces []NodeInterface
	Calls      []EdgeCalls
	DefinedIn  []EdgeDefinedIn
	Implements []EdgeImplements
	HasMethod  []EdgeHasMethod
}

// WriteReport summarises the result of a Write call.
type WriteReport struct {
	NodesWritten int
	EdgesWritten int
}

// writeJob is a single parameterised Cypher statement ready for execution.
type writeJob struct {
	cypher string
	params map[string]any
}

// ─── Writer ───────────────────────────────────────────────────────────────────

// Writer persists graph nodes and edges into Neo4j using MERGE semantics.
type Writer struct {
	cfg WriterConfig
	tx  txExecutor
}

// NewWriter creates a Writer backed by the given Neo4j driver.
func NewWriter(driver neo4j.DriverWithContext, cfg WriterConfig) *Writer {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 500
	}
	if cfg.Workers <= 0 {
		cfg.Workers = 4
	}
	return &Writer{cfg: cfg, tx: &neo4jTxExecutor{driver: driver}}
}

// Write upserts all nodes then all edges in batch into Neo4j.
// Nodes are written before edges to guarantee relationship endpoints exist.
func (w *Writer) Write(ctx context.Context, batch WriteBatch) (*WriteReport, error) {
	if err := w.writeAll(ctx, collectNodeJobs(batch, w.cfg.BatchSize)); err != nil {
		return nil, fmt.Errorf("writer: nodes: %w", err)
	}
	if err := w.writeAll(ctx, collectEdgeJobs(batch, w.cfg.BatchSize)); err != nil {
		return nil, fmt.Errorf("writer: edges: %w", err)
	}
	report := &WriteReport{
		NodesWritten: len(batch.Files) + len(batch.Functions) + len(batch.Methods) +
			len(batch.Classes) + len(batch.Structs) + len(batch.Interfaces),
		EdgesWritten: len(batch.Calls) + len(batch.DefinedIn) +
			len(batch.Implements) + len(batch.HasMethod),
	}
	slog.Info("neo4j write complete", "nodes", report.NodesWritten, "edges", report.EdgesWritten)
	return report, nil
}

// writeAll executes jobs concurrently using up to cfg.Workers goroutines.
// On the first error the context is cancelled so remaining jobs are skipped quickly.
func (w *Writer) writeAll(ctx context.Context, jobs []writeJob) error {
	if len(jobs) == 0 {
		return nil
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobCh := make(chan writeJob, len(jobs))
	for _, j := range jobs {
		jobCh <- j
	}
	close(jobCh)

	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		first error
	)
	for i := 0; i < w.cfg.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobCh {
				if ctx.Err() != nil {
					return
				}
				if err := w.tx.executeTx(ctx, job.cypher, job.params); err != nil {
					mu.Lock()
					if first == nil {
						first = err
					}
					mu.Unlock()
					cancel()
					return
				}
			}
		}()
	}
	wg.Wait()
	return first
}
