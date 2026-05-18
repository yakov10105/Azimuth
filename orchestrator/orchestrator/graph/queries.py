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
