from unittest.mock import AsyncMock, patch

import pytest
from fastapi.testclient import TestClient

from orchestrator.main import app

client = TestClient(app)

GRAPH_MODULE = "orchestrator.main.graph"


def _graph_result(
    summary: str = "PaymentHandler is in payments/handler.go",
    call_path: list[str] | None = None,
    relevant_files: list[str] | None = None,
    error: str | None = None,
) -> dict:
    result: dict = {
        "final_answer": summary,
        "call_path": call_path or ["HandlePayment (payments/handler.go:38)"],
        "relevant_files": relevant_files or ["payments/handler.go"],
    }
    if error:
        result["error"] = error
    return result


# ---------------------------------------------------------------------------
# /healthz
# ---------------------------------------------------------------------------

def test_healthz_returns_ok():
    response = client.get("/healthz")
    assert response.status_code == 200
    assert response.json() == {"status": "ok"}


# ---------------------------------------------------------------------------
# POST /ask — happy path
# ---------------------------------------------------------------------------

def test_ask_returns_answer():
    with patch(GRAPH_MODULE) as mock_graph:
        mock_graph.ainvoke = AsyncMock(return_value=_graph_result())
        response = client.post("/ask", json={"question": "Where is the payment handler?"})

    assert response.status_code == 200
    body = response.json()
    assert body["summary"] == "PaymentHandler is in payments/handler.go"
    assert body["call_path"] == ["HandlePayment (payments/handler.go:38)"]
    assert body["relevant_files"] == ["payments/handler.go"]


def test_ask_passes_question_and_depth_to_graph():
    with patch(GRAPH_MODULE) as mock_graph:
        mock_graph.ainvoke = AsyncMock(return_value=_graph_result())
        client.post("/ask", json={"question": "How does retry work?", "depth": 5})

    call_kwargs = mock_graph.ainvoke.call_args[0][0]
    assert call_kwargs["raw_query"] == "How does retry work?"
    assert call_kwargs["depth"] == 5


def test_ask_initialises_accumulated_lists_empty():
    with patch(GRAPH_MODULE) as mock_graph:
        mock_graph.ainvoke = AsyncMock(return_value=_graph_result())
        client.post("/ask", json={"question": "test"})

    initial_state = mock_graph.ainvoke.call_args[0][0]
    assert initial_state["retrieved_nodes"] == []
    assert initial_state["retrieved_edges"] == []
    assert initial_state["code_snippets"] == []
    assert initial_state["historical_context"] == []


# ---------------------------------------------------------------------------
# POST /ask — error cases
# ---------------------------------------------------------------------------

def test_ask_returns_500_on_graph_error():
    with patch(GRAPH_MODULE) as mock_graph:
        mock_graph.ainvoke = AsyncMock(
            return_value=_graph_result(error="Neo4j unreachable: connection refused")
        )
        response = client.post("/ask", json={"question": "test"})

    assert response.status_code == 500
    assert "Neo4j" in response.json()["detail"]


# ---------------------------------------------------------------------------
# Validation
# ---------------------------------------------------------------------------

def test_ask_validates_depth_bounds():
    # depth=0 is below minimum of 1
    response = client.post("/ask", json={"question": "test", "depth": 0})
    assert response.status_code == 422

    # depth=11 is above maximum of 10
    response = client.post("/ask", json={"question": "test", "depth": 11})
    assert response.status_code == 422
