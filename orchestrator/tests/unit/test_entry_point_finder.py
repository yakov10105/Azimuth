from unittest.mock import AsyncMock, patch

import pytest
from neo4j.exceptions import ServiceUnavailable

from orchestrator.nodes.entry_point_finder import entry_point_finder_node

MODULE = "orchestrator.nodes.entry_point_finder._neo4j_client"


def _make_record(fqn: str, name: str, in_degree: int = 0) -> dict:
    return {
        "fqn": fqn,
        "name": name,
        "file_path": f"pkg/{name.lower()}.go",
        "kind": "Function",
        "in_degree": in_degree,
        "out_degree": 1,
    }


def _mock_client(candidates: list[dict], entry_points: list[dict]) -> AsyncMock:
    client = AsyncMock()
    client.find_candidates.return_value = candidates
    client.find_entry_points.return_value = entry_points
    return client


# ---------------------------------------------------------------------------
# Happy path
# ---------------------------------------------------------------------------

@pytest.mark.asyncio
async def test_returns_candidates_for_entity():
    candidates = [_make_record("pkg.PaymentHandler", "PaymentHandler", in_degree=5)]
    ep = [_make_record("pkg.PaymentHandler", "PaymentHandler", in_degree=5)]
    with patch(MODULE, _mock_client(candidates, ep)):
        result = await entry_point_finder_node({"raw_query": "q", "entities": ["payment handler"]})
    assert result["entry_points"] == ep
    assert "error" not in result


@pytest.mark.asyncio
async def test_returns_top5_max():
    candidates = [_make_record(f"pkg.F{i}", f"F{i}", in_degree=i) for i in range(7)]
    top5 = candidates[:5]
    with patch(MODULE, _mock_client(candidates, top5)):
        result = await entry_point_finder_node({"raw_query": "q", "entities": ["handler"]})
    assert len(result["entry_points"]) == 5


@pytest.mark.asyncio
async def test_deduplicates_across_entities():
    shared = _make_record("pkg.Auth", "Auth", in_degree=10)
    # Both entity searches return the same FQN
    client = AsyncMock()
    client.find_candidates.return_value = [shared]
    client.find_entry_points.return_value = [shared]

    with patch(MODULE, client):
        result = await entry_point_finder_node(
            {"raw_query": "q", "entities": ["auth", "authentication"]}
        )

    # find_candidates called once per entity (2 times)
    assert client.find_candidates.call_count == 2
    # find_entry_points called with deduplicated list — only one unique FQN
    fqns_passed = client.find_entry_points.call_args.kwargs["fqns"]
    assert fqns_passed.count("pkg.Auth") == 1
    assert result["entry_points"] == [shared]


@pytest.mark.asyncio
async def test_ranks_by_in_degree():
    low = _make_record("pkg.Low", "Low", in_degree=1)
    high = _make_record("pkg.High", "High", in_degree=99)
    # find_entry_points returns them ranked (high first) — node trusts the DB ordering
    with patch(MODULE, _mock_client([low, high], [high, low])):
        result = await entry_point_finder_node({"raw_query": "q", "entities": ["handler"]})
    assert result["entry_points"][0]["fqn"] == "pkg.High"


# ---------------------------------------------------------------------------
# Empty / missing results
# ---------------------------------------------------------------------------

@pytest.mark.asyncio
async def test_empty_results_no_crash():
    with patch(MODULE, _mock_client([], [])):
        result = await entry_point_finder_node({"raw_query": "q", "entities": ["unknown"]})
    assert result["entry_points"] == []
    assert "error" in result


@pytest.mark.asyncio
async def test_no_entities_skips_neo4j():
    client = AsyncMock()
    with patch(MODULE, client):
        result = await entry_point_finder_node({"raw_query": "q", "entities": []})
    client.find_candidates.assert_not_called()
    assert result["entry_points"] == []
    assert "error" in result


@pytest.mark.asyncio
async def test_none_entities_skips_neo4j():
    client = AsyncMock()
    with patch(MODULE, client):
        result = await entry_point_finder_node({"raw_query": "q", "entities": None})
    client.find_candidates.assert_not_called()
    assert result["entry_points"] == []


# ---------------------------------------------------------------------------
# Error handling
# ---------------------------------------------------------------------------

@pytest.mark.asyncio
async def test_neo4j_unreachable_sets_error():
    client = AsyncMock()
    client.find_candidates.side_effect = ServiceUnavailable("connection refused")
    with patch(MODULE, client):
        result = await entry_point_finder_node({"raw_query": "q", "entities": ["handler"]})
    assert "error" in result
    assert "Neo4j" in result["error"]
    # Must not propagate — no exception raised
