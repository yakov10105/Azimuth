package graph

import (
	"fmt"
	"strings"
)

// collectNodeJobs builds write jobs for all node types in batch.
func collectNodeJobs(batch WriteBatch, batchSize int) []writeJob {
	var jobs []writeJob
	jobs = append(jobs, nodeFileJobs(batch.Files, batchSize)...)
	jobs = append(jobs, nodeFunctionJobs(batch.Functions, batchSize)...)
	jobs = append(jobs, nodeMethodJobs(batch.Methods, batchSize)...)
	jobs = append(jobs, nodeClassJobs(batch.Classes, batchSize)...)
	jobs = append(jobs, nodeStructJobs(batch.Structs, batchSize)...)
	jobs = append(jobs, nodeInterfaceJobs(batch.Interfaces, batchSize)...)
	return jobs
}

// collectEdgeJobs builds write jobs for all edge types in batch.
func collectEdgeJobs(batch WriteBatch, batchSize int) []writeJob {
	var jobs []writeJob
	jobs = append(jobs, edgeCallsJobs(batch.Calls, batchSize)...)
	jobs = append(jobs, edgeDefinedInJobs(batch.DefinedIn, batchSize)...)
	jobs = append(jobs, edgeImplementsJobs(batch.Implements, batchSize)...)
	jobs = append(jobs, edgeHasMethodJobs(batch.HasMethod, batchSize)...)
	return jobs
}

// chunkSlice splits items into consecutive chunks of at most size elements.
func chunkSlice[T any](items []T, size int) [][]T {
	if size <= 0 || len(items) == 0 {
		return nil
	}
	chunks := make([][]T, 0, (len(items)+size-1)/size)
	for i := 0; i < len(items); i += size {
		end := i + size
		if end > len(items) {
			end = len(items)
		}
		chunks = append(chunks, items[i:end])
	}
	return chunks
}

func nodeFileJobs(nodes []NodeFile, batchSize int) []writeJob {
	const cypher = `UNWIND $batch AS n
MERGE (f:File {path: n.path})
SET f.language    = n.language,
    f.package     = n.package,
    f.ingested_at = datetime()`
	var jobs []writeJob
	for _, chunk := range chunkSlice(nodes, batchSize) {
		batch := make([]map[string]any, len(chunk))
		for i, n := range chunk {
			batch[i] = map[string]any{"path": n.Path, "language": n.Language, "package": n.Package}
		}
		jobs = append(jobs, writeJob{cypher: cypher, params: map[string]any{"batch": batch}})
	}
	return jobs
}

func nodeFunctionJobs(nodes []NodeFunction, batchSize int) []writeJob {
	const cypher = `UNWIND $batch AS n
MERGE (f:Function {fqn: n.fqn})
SET f.name        = n.name,
    f.file_path   = n.file_path,
    f.start_line  = n.start_line,
    f.end_line    = n.end_line,
    f.language    = n.language,
    f.ingested_at = datetime()`
	var jobs []writeJob
	for _, chunk := range chunkSlice(nodes, batchSize) {
		batch := make([]map[string]any, len(chunk))
		for i, n := range chunk {
			batch[i] = map[string]any{
				"fqn": n.FQN, "name": n.Name, "file_path": n.FilePath,
				"start_line": n.StartLine, "end_line": n.EndLine, "language": n.Language,
			}
		}
		jobs = append(jobs, writeJob{cypher: cypher, params: map[string]any{"batch": batch}})
	}
	return jobs
}

func nodeMethodJobs(nodes []NodeMethod, batchSize int) []writeJob {
	const cypher = `UNWIND $batch AS n
MERGE (m:Method {fqn: n.fqn})
SET m.name        = n.name,
    m.receiver    = n.receiver,
    m.file_path   = n.file_path,
    m.start_line  = n.start_line,
    m.end_line    = n.end_line,
    m.ingested_at = datetime()`
	var jobs []writeJob
	for _, chunk := range chunkSlice(nodes, batchSize) {
		batch := make([]map[string]any, len(chunk))
		for i, n := range chunk {
			batch[i] = map[string]any{
				"fqn": n.FQN, "name": n.Name, "receiver": n.Receiver,
				"file_path": n.FilePath, "start_line": n.StartLine, "end_line": n.EndLine,
			}
		}
		jobs = append(jobs, writeJob{cypher: cypher, params: map[string]any{"batch": batch}})
	}
	return jobs
}

func nodeClassJobs(nodes []NodeClass, batchSize int) []writeJob {
	const cypher = `UNWIND $batch AS n
MERGE (c:Class {fqn: n.fqn})
SET c.name        = n.name,
    c.namespace   = n.namespace,
    c.file_path   = n.file_path,
    c.ingested_at = datetime()`
	var jobs []writeJob
	for _, chunk := range chunkSlice(nodes, batchSize) {
		batch := make([]map[string]any, len(chunk))
		for i, n := range chunk {
			batch[i] = map[string]any{
				"fqn": n.FQN, "name": n.Name, "namespace": n.Namespace, "file_path": n.FilePath,
			}
		}
		jobs = append(jobs, writeJob{cypher: cypher, params: map[string]any{"batch": batch}})
	}
	return jobs
}

func nodeStructJobs(nodes []NodeStruct, batchSize int) []writeJob {
	const cypher = `UNWIND $batch AS n
MERGE (s:Struct {fqn: n.fqn})
SET s.name        = n.name,
    s.package     = n.package,
    s.file_path   = n.file_path,
    s.ingested_at = datetime()`
	var jobs []writeJob
	for _, chunk := range chunkSlice(nodes, batchSize) {
		batch := make([]map[string]any, len(chunk))
		for i, n := range chunk {
			batch[i] = map[string]any{
				"fqn": n.FQN, "name": n.Name, "package": n.Package, "file_path": n.FilePath,
			}
		}
		jobs = append(jobs, writeJob{cypher: cypher, params: map[string]any{"batch": batch}})
	}
	return jobs
}

func nodeInterfaceJobs(nodes []NodeInterface, batchSize int) []writeJob {
	const cypher = `UNWIND $batch AS n
MERGE (i:Interface {fqn: n.fqn})
SET i.name                 = n.name,
    i.package_or_namespace = n.package_or_namespace,
    i.file_path            = n.file_path,
    i.ingested_at          = datetime()`
	var jobs []writeJob
	for _, chunk := range chunkSlice(nodes, batchSize) {
		batch := make([]map[string]any, len(chunk))
		for i, n := range chunk {
			batch[i] = map[string]any{
				"fqn": n.FQN, "name": n.Name,
				"package_or_namespace": n.PackageOrNamespace, "file_path": n.FilePath,
			}
		}
		jobs = append(jobs, writeJob{cypher: cypher, params: map[string]any{"batch": batch}})
	}
	return jobs
}

func edgeCallsJobs(edges []EdgeCalls, batchSize int) []writeJob {
	const cypher = `UNWIND $batch AS e
MATCH (caller) WHERE (caller:Function OR caller:Method) AND caller.fqn = e.caller_fqn
MATCH (callee) WHERE (callee:Function OR callee:Method) AND callee.fqn = e.callee_fqn
MERGE (caller)-[:CALLS {call_site_file: e.call_site_file, call_site_line: e.call_site_line}]->(callee)`
	resolved := make([]EdgeCalls, 0, len(edges))
	for _, e := range edges {
		if !strings.HasPrefix(e.CalleeFQN, "EXTERNAL::") {
			resolved = append(resolved, e)
		}
	}
	var jobs []writeJob
	for _, chunk := range chunkSlice(resolved, batchSize) {
		batch := make([]map[string]any, len(chunk))
		for i, e := range chunk {
			batch[i] = map[string]any{
				"caller_fqn": e.CallerFQN, "callee_fqn": e.CalleeFQN,
				"call_site_file": e.CallSiteFile, "call_site_line": e.CallSiteLine,
			}
		}
		jobs = append(jobs, writeJob{cypher: cypher, params: map[string]any{"batch": batch}})
	}
	return jobs
}

// edgeDefinedInJobs groups edges by NodeLabel and generates one query per label.
// This avoids ambiguous MATCH (n) WHERE n.fqn = ... across multiple labelled types.
func edgeDefinedInJobs(edges []EdgeDefinedIn, batchSize int) []writeJob {
	byLabel := make(map[string][]EdgeDefinedIn)
	for _, e := range edges {
		byLabel[e.NodeLabel] = append(byLabel[e.NodeLabel], e)
	}
	var jobs []writeJob
	for label, group := range byLabel {
		cypher := fmt.Sprintf(`UNWIND $batch AS e
MATCH (n:%s {fqn: e.node_fqn})
MATCH (f:File {path: e.file_path})
MERGE (n)-[:DEFINED_IN]->(f)`, label)
		for _, chunk := range chunkSlice(group, batchSize) {
			batch := make([]map[string]any, len(chunk))
			for i, e := range chunk {
				batch[i] = map[string]any{"node_fqn": e.NodeFQN, "file_path": e.FilePath}
			}
			jobs = append(jobs, writeJob{cypher: cypher, params: map[string]any{"batch": batch}})
		}
	}
	return jobs
}

func edgeImplementsJobs(edges []EdgeImplements, batchSize int) []writeJob {
	const cypher = `UNWIND $batch AS e
MATCH (impl) WHERE (impl:Class OR impl:Struct) AND impl.fqn = e.implementor_fqn
MATCH (iface:Interface {fqn: e.interface_fqn})
MERGE (impl)-[:IMPLEMENTS]->(iface)`
	var jobs []writeJob
	for _, chunk := range chunkSlice(edges, batchSize) {
		batch := make([]map[string]any, len(chunk))
		for i, e := range chunk {
			batch[i] = map[string]any{"implementor_fqn": e.ImplementorFQN, "interface_fqn": e.InterfaceFQN}
		}
		jobs = append(jobs, writeJob{cypher: cypher, params: map[string]any{"batch": batch}})
	}
	return jobs
}

func edgeHasMethodJobs(edges []EdgeHasMethod, batchSize int) []writeJob {
	const cypher = `UNWIND $batch AS e
MATCH (owner) WHERE (owner:Class OR owner:Struct) AND owner.fqn = e.owner_fqn
MATCH (m:Method {fqn: e.method_fqn})
MERGE (owner)-[:HAS_METHOD]->(m)`
	var jobs []writeJob
	for _, chunk := range chunkSlice(edges, batchSize) {
		batch := make([]map[string]any, len(chunk))
		for i, e := range chunk {
			batch[i] = map[string]any{"owner_fqn": e.OwnerFQN, "method_fqn": e.MethodFQN}
		}
		jobs = append(jobs, writeJob{cypher: cypher, params: map[string]any{"batch": batch}})
	}
	return jobs
}
