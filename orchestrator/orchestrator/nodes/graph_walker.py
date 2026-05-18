import logging

from neo4j.exceptions import ServiceUnavailable

from ..config import GRAPH_WALKER_DEPTH, NEO4J_PASSWORD, NEO4J_URI, NEO4J_USER
from ..graph.client import Neo4jReadClient
from ..state import AzimuthState

logger = logging.getLogger(__name__)

_CODE_CAP = 50_000  # maximum total characters of source code returned to the LLM

# Module-level singleton — replace in unit tests
_neo4j_client: Neo4jReadClient = Neo4jReadClient(NEO4J_URI, NEO4J_USER, NEO4J_PASSWORD)


def _read_snippet(file_path: str, start_line: int, end_line: int) -> str | None:
    """Read lines [start_line, end_line] (1-indexed, inclusive) from a source file."""
    try:
        with open(file_path, encoding="utf-8", errors="replace") as fh:
            lines = fh.readlines()
        start = max(0, start_line - 1)
        end = min(len(lines), end_line)
        return "".join(lines[start:end])
    except OSError:
        return None


async def graph_walker_node(state: AzimuthState) -> dict:
    entry_points: list[dict] = state.get("entry_points") or []

    if not entry_points:
        logger.warning("graph_walker received empty entry_points")
        return {
            "retrieved_nodes": [],
            "retrieved_edges": [],
            "code_snippets": [],
            "error": "No entry points to walk from",
        }

    depth: int = state.get("depth") or GRAPH_WALKER_DEPTH

    try:
        # Collect subgraph across all entry points, deduplicating by FQN / edge key
        seen_nodes: dict[str, dict] = {}
        seen_edges: dict[tuple, dict] = {}

        for ep in entry_points:
            fqn = ep.get("fqn")
            if not fqn:
                continue
            nodes, edges = await _neo4j_client.get_subgraph(fqn, depth)
            for node in nodes:
                node_fqn = node.get("fqn")
                if node_fqn and (
                    node_fqn not in seen_nodes
                    or node.get("in_degree", 0) > seen_nodes[node_fqn].get("in_degree", 0)
                ):
                    seen_nodes[node_fqn] = node
            for edge in edges:
                key = (edge.get("from_fqn"), edge.get("to_fqn"), edge.get("type"))
                seen_edges[key] = edge

    except ServiceUnavailable as exc:
        logger.error("graph_walker Neo4j unreachable: %s", exc)
        return {"retrieved_nodes": [], "retrieved_edges": [], "code_snippets": [],
                "error": f"Neo4j unreachable: {exc}"}

    # Sort most-central nodes first so truncation removes the least-relevant ones
    sorted_nodes = sorted(
        seen_nodes.values(),
        key=lambda n: n.get("in_degree", 0),
        reverse=True,
    )

    # Read source snippets, stopping once the cap is reached
    snippets: list[dict] = []
    total_chars = 0
    for node in sorted_nodes:
        fp = node.get("file_path")
        sl = node.get("start_line")
        el = node.get("end_line")
        if not (fp and sl and el):
            continue
        content = _read_snippet(fp, sl, el)
        if not content:
            continue
        if total_chars + len(content) > _CODE_CAP:
            logger.debug(
                "graph_walker snippet cap reached at %d chars; dropping %s", total_chars, fp
            )
            break
        snippets.append({"file": fp, "start_line": sl, "end_line": el, "content": content})
        total_chars += len(content)

    logger.debug(
        "graph_walker nodes=%d edges=%d snippets=%d total_chars=%d",
        len(sorted_nodes), len(seen_edges), len(snippets), total_chars,
    )
    return {
        "retrieved_nodes": sorted_nodes,
        "retrieved_edges": list(seen_edges.values()),
        "code_snippets": snippets,
    }
