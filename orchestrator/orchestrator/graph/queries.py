# Cypher query library — read-only queries used by the orchestrator.
# All values are parameterised — never interpolate strings into Cypher.

FIND_BY_NAME = """
MATCH (n)
WHERE (n:Function OR n:Method OR n:Class OR n:Struct OR n:Interface)
  AND toLower(n.name) CONTAINS toLower($name)
RETURN n.fqn       AS fqn,
       n.name      AS name,
       n.file_path AS file_path,
       labels(n)[0] AS kind,
       size([(n)<-[:CALLS]-() | 1]) AS in_degree
ORDER BY in_degree DESC
LIMIT 10
"""

FIND_ENTRY_POINTS = """
MATCH (n)
WHERE (n:Function OR n:Method OR n:Class OR n:Struct OR n:Interface)
  AND n.fqn IN $fqns
WITH n,
     size([(n)<-[:CALLS]-() | 1]) AS in_degree,
     size([(n)-[:CALLS]->()  | 1]) AS out_degree
RETURN n.fqn       AS fqn,
       n.name      AS name,
       n.file_path AS file_path,
       labels(n)[0] AS kind,
       in_degree,
       out_degree
ORDER BY in_degree DESC
LIMIT $limit
"""

GET_SUBGRAPH = """
MATCH (root {fqn: $fqn})
CALL apoc.path.subgraphAll(root, {
  relationshipFilter: "CALLS>|IMPLEMENTS>",
  maxLevel: $depth
})
YIELD nodes AS raw_nodes, relationships
RETURN [n IN raw_nodes | {
  fqn:        n.fqn,
  name:       n.name,
  file_path:  n.file_path,
  start_line: n.start_line,
  end_line:   n.end_line,
  kind:       labels(n)[0],
  in_degree:  size([(n)<-[:CALLS]-() | 1])
}] AS nodes,
[r IN relationships | {
  from_fqn: startNode(r).fqn,
  to_fqn:   endNode(r).fqn,
  type:     type(r)
}] AS edges
"""
