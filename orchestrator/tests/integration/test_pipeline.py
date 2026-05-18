"""
Integration tests for the Azimuth orchestrator pipeline.

Requires:
  - ANTHROPIC_API_KEY set in the environment
  - Neo4j running (for full pipeline tests added in Task 1.5.6)

Run with:
  python -m pytest tests/integration/ -v -m integration
"""
import os
import time

import pytest

from orchestrator.nodes.synthesis import synthesis_node

pytestmark = pytest.mark.integration


def _has_api_key() -> bool:
    return bool(os.getenv("ANTHROPIC_API_KEY"))


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
            # Must contain a file path reference in parentheses
            assert "(" in entry and ")" in entry, (
                f"call_path entry missing file reference: {entry!r}"
            )
