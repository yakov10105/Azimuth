from unittest.mock import AsyncMock, MagicMock, patch

import pytest
from neo4j.exceptions import ServiceUnavailable

from orchestrator.nodes.graph_walker import graph_walker_node

NODE_MODULE = "orchestrator.nodes.graph_walker._neo4j_client"
SNIPPET_FN = "orchestrator.nodes.graph_walker._read_snippet"


def _node(fqn: str, name: str, in_degree: int = 1, file_path: str = "/repo/a.go",
          start_line: int = 1, end_line: int = 10) -> dict:
    return {
        "fqn": fqn, "name": name, "file_path": file_path,
        "start_line": start_line, "end_line": end_line,
        "kind": "Function", "in_degree": in_degree,
    }


def _edge(from_fqn: str, to_fqn: str) -> dict:
    return {"from_fqn": from_fqn, "to_fqn": to_fqn, "type": "CALLS"}


def _mock_client(nodes: list[dict], edges: list[dict]) -> AsyncMock:
    client = AsyncMock()
    client.get_subgraph.return_value = (nodes, edges)
    return client


def _state(**kwargs) -> dict:
    base = {"raw_query": "q", "depth": 3,
            "entry_points": [{"fqn": "pkg.Handler", "name": "Handler"}]}
    base.update(kwargs)
    return base


# ---------------------------------------------------------------------------
# Happy path
# ---------------------------------------------------------------------------

@pytest.mark.asyncio
async def test_returns_nodes_and_edges():
    nodes = [_node("pkg.A", "A"), _node("pkg.B", "B")]
    edges = [_edge("pkg.A", "pkg.B")]
    with patch(NODE_MODULE, _mock_client(nodes, edges)), \
         patch(SNIPPET_FN, return_value="code"):
        result = await graph_walker_node(_state())
    assert len(result["retrieved_nodes"]) == 2
    assert len(result["retrieved_edges"]) == 1


@pytest.mark.asyncio
async def test_reads_code_snippets():
    nodes = [_node("pkg.A", "A", file_path="/repo/a.go", start_line=1, end_line=5)]
    with patch(NODE_MODULE, _mock_client(nodes, [])), \
         patch(SNIPPET_FN, return_value="func A() {}"):
        result = await graph_walker_node(_state())
    assert len(result["code_snippets"]) == 1
    assert result["code_snippets"][0]["content"] == "func A() {}"
    assert result["code_snippets"][0]["file"] == "/repo/a.go"


@pytest.mark.asyncio
async def test_caps_snippets_at_50000_chars():
    big = "x" * 30_000
    nodes = [
        _node("pkg.A", "A", in_degree=10, file_path="/a.go"),
        _node("pkg.B", "B", in_degree=1,  file_path="/b.go"),
    ]

    def _fake_read(fp, sl, el):
        return big  # every file returns 30k chars

    with patch(NODE_MODULE, _mock_client(nodes, [])), \
         patch(SNIPPET_FN, side_effect=_fake_read):
        result = await graph_walker_node(_state())

    # First node (30k) fits; second would push to 60k — must be dropped
    assert len(result["code_snippets"]) == 1
    assert result["code_snippets"][0]["file"] == "/a.go"


@pytest.mark.asyncio
async def test_depth_passed_to_client():
    client = AsyncMock()
    client.get_subgraph.return_value = ([], [])
    with patch(NODE_MODULE, client), patch(SNIPPET_FN, return_value=None):
        await graph_walker_node(_state(depth=5))
    client.get_subgraph.assert_called_once_with("pkg.Handler", 5)


@pytest.mark.asyncio
async def test_default_depth_used_when_none():
    from orchestrator.config import GRAPH_WALKER_DEPTH
    client = AsyncMock()
    client.get_subgraph.return_value = ([], [])
    with patch(NODE_MODULE, client), patch(SNIPPET_FN, return_value=None):
        await graph_walker_node(_state(depth=None))
    _, called_depth = client.get_subgraph.call_args[0]
    assert called_depth == GRAPH_WALKER_DEPTH


@pytest.mark.asyncio
async def test_deduplicates_nodes_across_entry_points():
    shared = _node("pkg.Shared", "Shared", in_degree=5)
    unique = _node("pkg.Unique", "Unique", in_degree=1)
    client = AsyncMock()
    # Two entry points, both return the shared node; second also returns unique
    client.get_subgraph.side_effect = [
        ([shared], []),
        ([shared, unique], []),
    ]
    eps = [{"fqn": "pkg.A", "name": "A"}, {"fqn": "pkg.B", "name": "B"}]
    with patch(NODE_MODULE, client), patch(SNIPPET_FN, return_value=None):
        result = await graph_walker_node(_state(entry_points=eps))
    fqns = [n["fqn"] for n in result["retrieved_nodes"]]
    assert fqns.count("pkg.Shared") == 1
    assert len(fqns) == 2


@pytest.mark.asyncio
async def test_most_central_node_first():
    low  = _node("pkg.Low",  "Low",  in_degree=1)
    high = _node("pkg.High", "High", in_degree=99)
    with patch(NODE_MODULE, _mock_client([low, high], [])), \
         patch(SNIPPET_FN, return_value=None):
        result = await graph_walker_node(_state())
    assert result["retrieved_nodes"][0]["fqn"] == "pkg.High"


# ---------------------------------------------------------------------------
# Empty / error cases
# ---------------------------------------------------------------------------

@pytest.mark.asyncio
async def test_empty_entry_points_returns_error():
    client = AsyncMock()
    with patch(NODE_MODULE, client):
        result = await graph_walker_node(_state(entry_points=[]))
    client.get_subgraph.assert_not_called()
    assert result["retrieved_nodes"] == []
    assert "error" in result


@pytest.mark.asyncio
async def test_neo4j_unreachable_sets_error():
    client = AsyncMock()
    client.get_subgraph.side_effect = ServiceUnavailable("refused")
    with patch(NODE_MODULE, client):
        result = await graph_walker_node(_state())
    assert "error" in result
    assert "Neo4j" in result["error"]
    assert result["retrieved_nodes"] == []
