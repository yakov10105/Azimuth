"""
Integration tests for the Azimuth orchestrator pipeline.

Requires:
  - ANTHROPIC_API_KEY set in the environment
  - Neo4j running at NEO4J_URI (for full pipeline graph tests)

Run with:
  python -m pytest tests/integration/ -v -m integration
"""
import os
import time

import pytest

from orchestrator.agent import graph
from orchestrator.nodes.synthesis import synthesis_node

pytestmark = pytest.mark.integration


def _has_api_key() -> bool:
    return bool(os.getenv("ANTHROPIC_API_KEY"))


def _has_neo4j() -> bool:
    return bool(os.getenv("NEO4J_URI") or os.getenv("NEO4J_PASSWORD"))


@pytest.fixture
def payment_handler_state() -> dict:
    """Minimal seeded state simulating 'Where is the payment handler?' query."""
    return {
        "raw_query": "Where is the payment handler?",
        "depth": 3,
        "retrieved_nodes": [
            {
                "fqn": "payments.HandlePayment",
                "name": "HandlePayment",
                "file_path": "payments/handler.go",
                "kind": "Function",
                "in_degree": 5,
            }
        ],
        "retrieved_edges": [
            {
                "from_fqn": "main.RegisterRoutes",
                "to_fqn": "payments.HandlePayment",
                "type": "CALLS",
            }
        ],
        "code_snippets": [
            {
                "file": "payments/handler.go",
                "start_line": 38,
                "end_line": 60,
                "content": (
                    "// HandlePayment processes an incoming payment request.\n"
                    "func HandlePayment(w http.ResponseWriter, r *http.Request) {\n"
                    "    req, err := parsePaymentRequest(r)\n"
                    "    if err != nil {\n"
                    "        http.Error(w, err.Error(), http.StatusBadRequest)\n"
                    "        return\n"
                    "    }\n"
                    "    result, err := paymentService.Process(r.Context(), req)\n"
                    "    if err != nil {\n"
                    "        http.Error(w, err.Error(), http.StatusInternalServerError)\n"
                    "        return\n"
                    "    }\n"
                    "    json.NewEncoder(w).Encode(result)\n"
                    "}\n"
                ),
            }
        ],
    }


@pytest.mark.skipif(not _has_api_key(), reason="ANTHROPIC_API_KEY not set")
def test_synthesis_node_answers_where_query(payment_handler_state):
    """synthesis_node returns a non-empty answer citing the correct file."""
    start = time.perf_counter()
    result = synthesis_node(payment_handler_state)
    elapsed = time.perf_counter() - start

    assert "error" not in result, f"synthesis_node returned error: {result.get('error')}"
    assert result.get("final_answer"), "final_answer should be non-empty"
    assert result.get("relevant_files"), "relevant_files should be non-empty"
    assert any(
        "payments/handler.go" in f for f in result["relevant_files"]
    ), f"Expected payments/handler.go in relevant_files, got: {result['relevant_files']}"

    # Synthesis node alone should complete well within the 5s pipeline budget
    assert elapsed < 5.0, f"synthesis_node took {elapsed:.2f}s — exceeds 5s budget"


@pytest.mark.skipif(not _has_api_key(), reason="ANTHROPIC_API_KEY not set")
def test_synthesis_node_call_path_format(payment_handler_state):
    """call_path entries follow the 'FunctionName (file.go:N)' format."""
    result = synthesis_node(payment_handler_state)
    if result.get("call_path"):
        for entry in result["call_path"]:
            assert "(" in entry and ")" in entry, (
                f"call_path entry missing file reference: {entry!r}"
            )


# ---------------------------------------------------------------------------
# Full pipeline graph tests (requires Neo4j + Anthropic API key)
# ---------------------------------------------------------------------------

@pytest.mark.skipif(
    not (_has_api_key() and _has_neo4j()),
    reason="ANTHROPIC_API_KEY and NEO4J_URI/NEO4J_PASSWORD required",
)
@pytest.mark.asyncio
async def test_full_graph_with_seeded_state(payment_handler_state):
    """
    Invokes the compiled graph with a pre-populated state (bypassing nodes 1-3)
    by injecting context directly. Asserts AskResponse fields are non-empty.

    For a true end-to-end test (nodes 1→2→3→4 against live Neo4j), ingest a
    fixture repo first, then invoke with only raw_query and depth set.
    """
    # Seed the graph state past node 1/2/3 output — tests synthesis integration
    initial_state = {
        "raw_query": payment_handler_state["raw_query"],
        "depth": payment_handler_state["depth"],
        "retrieved_nodes": payment_handler_state["retrieved_nodes"],
        "retrieved_edges": payment_handler_state["retrieved_edges"],
        "code_snippets": payment_handler_state["code_snippets"],
        "historical_context": [],
        # Pre-set node 1/2 outputs so walk_graph/synthesize receive context
        "entities": ["payment handler"],
        "intent": "Locate the payment handler",
        "query_type": "WHERE",
        "entry_points": [payment_handler_state["retrieved_nodes"][0]],
    }

    start = time.perf_counter()
    result = await graph.ainvoke(initial_state)
    elapsed = time.perf_counter() - start

    assert result.get("final_answer"), "final_answer should be non-empty"
    assert result.get("relevant_files"), "relevant_files should be non-empty"
    assert elapsed < 5.0, f"Full graph took {elapsed:.2f}s — exceeds 5s budget"
