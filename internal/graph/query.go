package graph

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// ─── Return types ─────────────────────────────────────────────────────────────

// Node represents a graph node returned by a query.
type Node struct {
	ID         int64
	Labels     []string
	Properties map[string]any
}

// Edge represents a directed relationship between two graph nodes.
type Edge struct {
	Type       string
	Properties map[string]any
	StartFQN   string
	EndFQN     string
}

// Subgraph holds the nodes and edges returned by a traversal query.
type Subgraph struct {
	Nodes []Node
	Edges []Edge
}

// ─── QueryClient interface ────────────────────────────────────────────────────

// QueryClient is the read interface used by the orchestrator to traverse the graph.
// Defined here, next to its implementation; imported by internal/orchestrator.
type QueryClient interface {
	// FindByName returns up to 10 code entity nodes whose name contains the given
	// string (case-insensitive), ranked by call-site in-degree.
	FindByName(ctx context.Context, name string) ([]Node, error)

	// FindCallers returns all CALLS edges on inbound paths to fqn up to depth hops.
	// depth is clamped to [1, 5].
	FindCallers(ctx context.Context, fqn string, depth int) ([]Edge, error)

	// FindCallees returns all CALLS edges on outbound paths from fqn up to depth hops.
	// depth is clamped to [1, 5].
	FindCallees(ctx context.Context, fqn string, depth int) ([]Edge, error)

	// FindImplementors returns all Class and Struct nodes that implement the interface
	// identified by interfaceFQN.
	FindImplementors(ctx context.Context, interfaceFQN string) ([]Node, error)

	// FindEntryPoints returns up to 10 code entity nodes whose name contains any of
	// the given keywords, ranked by call-site in-degree. Returns an empty slice when
	// keywords is empty without hitting the database.
	FindEntryPoints(ctx context.Context, keywords []string) ([]Node, error)

	// GetSubgraph returns all nodes and CALLS/IMPLEMENTS edges reachable from
	// rootFQN within depth hops. The root is always included even if it has no edges.
	// depth is clamped to [1, 5].
	GetSubgraph(ctx context.Context, rootFQN string, depth int) (*Subgraph, error)
}

// ─── Read executor seam ───────────────────────────────────────────────────────

// queryRow is a single record from a read query, keyed by column name.
type queryRow = map[string]any

// readExecutor abstracts Neo4j session reads, allowing unit tests to inject stubs.
type readExecutor interface {
	executeRead(ctx context.Context, cypher string, params map[string]any) ([]queryRow, error)
}

// neo4jReadExecutor is the production readExecutor backed by a real Neo4j driver.
type neo4jReadExecutor struct {
	driver neo4j.DriverWithContext
}

func (e *neo4jReadExecutor) executeRead(ctx context.Context, cypher string, params map[string]any) ([]queryRow, error) {
	session := e.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)
	result, err := session.Run(ctx, cypher, params)
	if err != nil {
		return nil, err
	}
	collected, err := result.Collect(ctx)
	if err != nil {
		return nil, err
	}
	rows := make([]queryRow, len(collected))
	for i, rec := range collected {
		row := make(queryRow, len(rec.Keys))
		for _, key := range rec.Keys {
			val, _ := rec.Get(key)
			row[key] = val
		}
		rows[i] = row
	}
	return rows, nil
}

// ─── Neo4jQuery ───────────────────────────────────────────────────────────────

// Neo4jQuery implements QueryClient against a live Neo4j instance.
type Neo4jQuery struct {
	runner readExecutor
}

// NewNeo4jQuery creates a Neo4jQuery backed by the given driver.
func NewNeo4jQuery(driver neo4j.DriverWithContext) *Neo4jQuery {
	return &Neo4jQuery{runner: &neo4jReadExecutor{driver: driver}}
}

func (q *Neo4jQuery) FindByName(ctx context.Context, name string) ([]Node, error) {
	const cypher = `MATCH (n)
WHERE (n:Function OR n:Method OR n:Class OR n:Struct OR n:Interface)
  AND toLower(n.name) CONTAINS toLower($name)
WITH n, size([(n)<-[:CALLS]-() | 1]) AS in_degree
ORDER BY in_degree DESC
RETURN n
LIMIT 10`
	rows, err := q.runner.executeRead(ctx, cypher, map[string]any{"name": name})
	if err != nil {
		return nil, fmt.Errorf("FindByName: %w", err)
	}
	return rowsToNodes(rows, "n"), nil
}

func (q *Neo4jQuery) FindCallers(ctx context.Context, fqn string, depth int) ([]Edge, error) {
	depth = clampDepth(depth)
	const cypher = `MATCH (target)
WHERE (target:Function OR target:Method) AND target.fqn = $fqn
MATCH path = (caller)-[:CALLS*1..$depth]->(target)
UNWIND relationships(path) AS r
RETURN DISTINCT startNode(r) AS start, r AS rel, endNode(r) AS end`
	rows, err := q.runner.executeRead(ctx, cypher, map[string]any{"fqn": fqn, "depth": depth})
	if err != nil {
		return nil, fmt.Errorf("FindCallers: %w", err)
	}
	return rowsToEdges(rows), nil
}

func (q *Neo4jQuery) FindCallees(ctx context.Context, fqn string, depth int) ([]Edge, error) {
	depth = clampDepth(depth)
	const cypher = `MATCH (root)
WHERE (root:Function OR root:Method) AND root.fqn = $fqn
MATCH path = (root)-[:CALLS*1..$depth]->(callee)
UNWIND relationships(path) AS r
RETURN DISTINCT startNode(r) AS start, r AS rel, endNode(r) AS end`
	rows, err := q.runner.executeRead(ctx, cypher, map[string]any{"fqn": fqn, "depth": depth})
	if err != nil {
		return nil, fmt.Errorf("FindCallees: %w", err)
	}
	return rowsToEdges(rows), nil
}

func (q *Neo4jQuery) FindImplementors(ctx context.Context, interfaceFQN string) ([]Node, error) {
	const cypher = `MATCH (impl)-[:IMPLEMENTS]->(iface:Interface {fqn: $fqn})
RETURN impl AS n
UNION
MATCH (impl)-[:INJECTS]->(iface:Interface {fqn: $fqn})
RETURN impl AS n`
	rows, err := q.runner.executeRead(ctx, cypher, map[string]any{"fqn": interfaceFQN})
	if err != nil {
		return nil, fmt.Errorf("FindImplementors: %w", err)
	}
	return rowsToNodes(rows, "n"), nil
}

func (q *Neo4jQuery) FindEntryPoints(ctx context.Context, keywords []string) ([]Node, error) {
	if len(keywords) == 0 {
		return []Node{}, nil
	}
	const cypher = `MATCH (n)
WHERE (n:Function OR n:Method OR n:Class OR n:Struct OR n:Interface)
  AND ANY(k IN $keywords WHERE toLower(n.name) CONTAINS toLower(k))
WITH n, size([(n)<-[:CALLS]-() | 1]) AS in_degree
ORDER BY in_degree DESC
RETURN n
LIMIT 10`
	rows, err := q.runner.executeRead(ctx, cypher, map[string]any{"keywords": keywords})
	if err != nil {
		return nil, fmt.Errorf("FindEntryPoints: %w", err)
	}
	return rowsToNodes(rows, "n"), nil
}

// GetSubgraph makes two sequential reads: one to fetch the root node and one to
// traverse outward. Two calls keep the Cypher simple without requiring APOC.
func (q *Neo4jQuery) GetSubgraph(ctx context.Context, rootFQN string, depth int) (*Subgraph, error) {
	depth = clampDepth(depth)
	nodeSet := make(map[string]Node)
	var edges []Edge

	rootRows, err := q.runner.executeRead(ctx,
		`MATCH (n {fqn: $fqn}) RETURN n LIMIT 1`,
		map[string]any{"fqn": rootFQN})
	if err != nil {
		return nil, fmt.Errorf("GetSubgraph: root lookup: %w", err)
	}
	if len(rootRows) == 1 {
		if n, err := rowToNode(rootRows[0]["n"]); err == nil {
			nodeSet[rootFQN] = n
		}
	}

	const traverseCypher = `MATCH path = (root {fqn: $fqn})-[:CALLS|IMPLEMENTS*1..$depth]->(n)
UNWIND relationships(path) AS r
RETURN DISTINCT startNode(r) AS start, r AS rel, endNode(r) AS end`
	rows, err := q.runner.executeRead(ctx, traverseCypher, map[string]any{"fqn": rootFQN, "depth": depth})
	if err != nil {
		return nil, fmt.Errorf("GetSubgraph: traverse: %w", err)
	}
	for _, row := range rows {
		start, sErr := rowToNode(row["start"])
		end, eErr := rowToNode(row["end"])
		rel, rErr := rowToEdge(row["rel"])
		if sErr != nil || eErr != nil || rErr != nil {
			continue
		}
		startFQN, _ := start.Properties["fqn"].(string)
		endFQN, _ := end.Properties["fqn"].(string)
		nodeSet[startFQN] = start
		nodeSet[endFQN] = end
		rel.StartFQN = startFQN
		rel.EndFQN = endFQN
		edges = append(edges, rel)
	}

	nodes := make([]Node, 0, len(nodeSet))
	for _, n := range nodeSet {
		nodes = append(nodes, n)
	}
	return &Subgraph{Nodes: nodes, Edges: edges}, nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// clampDepth keeps depth within [1, 5] to prevent unbounded graph traversals.
func clampDepth(d int) int {
	if d < 1 {
		return 1
	}
	if d > 5 {
		return 5
	}
	return d
}

func rowToNode(val any) (Node, error) {
	n, ok := val.(neo4j.Node)
	if !ok {
		return Node{}, fmt.Errorf("expected neo4j.Node, got %T", val)
	}
	return Node{ID: n.Id, Labels: n.Labels, Properties: n.Props}, nil
}

func rowToEdge(val any) (Edge, error) {
	r, ok := val.(neo4j.Relationship)
	if !ok {
		return Edge{}, fmt.Errorf("expected neo4j.Relationship, got %T", val)
	}
	return Edge{Type: r.Type, Properties: r.Props}, nil
}

func rowsToNodes(rows []queryRow, key string) []Node {
	nodes := make([]Node, 0, len(rows))
	for _, row := range rows {
		n, err := rowToNode(row[key])
		if err != nil {
			continue
		}
		nodes = append(nodes, n)
	}
	return nodes
}

func rowsToEdges(rows []queryRow) []Edge {
	edges := make([]Edge, 0, len(rows))
	for _, row := range rows {
		start, sErr := rowToNode(row["start"])
		end, eErr := rowToNode(row["end"])
		rel, rErr := rowToEdge(row["rel"])
		if sErr != nil || eErr != nil || rErr != nil {
			continue
		}
		startFQN, _ := start.Properties["fqn"].(string)
		endFQN, _ := end.Properties["fqn"].(string)
		rel.StartFQN = startFQN
		rel.EndFQN = endFQN
		edges = append(edges, rel)
	}
	return edges
}
