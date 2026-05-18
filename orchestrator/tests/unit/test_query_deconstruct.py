import json
from unittest.mock import MagicMock, patch

import pytest

from orchestrator.nodes.query_deconstruct import query_deconstruct_node

MODULE = "orchestrator.nodes.query_deconstruct._llm_client"


def _mock_llm(entities: list[str], intent: str, query_type: str) -> MagicMock:
    m = MagicMock()
    m.complete.return_value = json.dumps(
        {"entities": entities, "intent": intent, "query_type": query_type}
    )
    return m


# ---------------------------------------------------------------------------
# 10 representative query tests
# ---------------------------------------------------------------------------

@pytest.mark.parametrize("query,entities,intent,query_type", [
    (
        "Where is the payment handler?",
        ["payment handler"],
        "Locate the payment handler",
        "WHERE",
    ),
    (
        "Where is the authentication middleware defined?",
        ["authentication middleware"],
        "Find the authentication middleware definition",
        "WHERE",
    ),
    (
        "How does the retry logic work in the order service?",
        ["retry logic", "order service"],
        "Understand retry logic in the order service",
        "HOW",
    ),
    (
        "How does the database connection pool get initialised?",
        ["database connection pool"],
        "Understand the database connection pool initialisation",
        "HOW",
    ),
    (
        "Why is there a mutex around the user cache?",
        ["mutex", "user cache"],
        "Understand why the user cache is locked",
        "WHY",
    ),
    (
        "Why does the config loader read from environment variables last?",
        ["config loader", "environment variables"],
        "Understand config loading order rationale",
        "WHY",
    ),
    (
        "If I change the User struct, what will break?",
        ["User"],
        "Identify components affected by changes to User struct",
        "IMPACT",
    ),
    (
        "What are the downstream effects of removing the caching layer?",
        ["caching layer"],
        "Identify downstream effects of removing the caching layer",
        "IMPACT",
    ),
    (
        "Where is the error handler for HTTP 500 responses?",
        ["error handler", "HTTP 500"],
        "Locate the HTTP 500 error handler",
        "WHERE",
    ),
    (
        "How does the ingestion pipeline process Go files?",
        ["ingestion pipeline", "Go files"],
        "Understand how Go files are processed in the ingestion pipeline",
        "HOW",
    ),
])
def test_representative_queries(query, entities, intent, query_type):
    with patch(MODULE, _mock_llm(entities, intent, query_type)):
        result = query_deconstruct_node({"raw_query": query})

    assert result["query_type"] == query_type
    assert isinstance(result["entities"], list)
    assert len(result["entities"]) > 0
    assert isinstance(result["intent"], str)
    assert result["intent"]


# ---------------------------------------------------------------------------
# Fallback tests
# ---------------------------------------------------------------------------

def test_fallback_on_malformed_json():
    mock = MagicMock()
    mock.complete.return_value = "not valid json at all"
    with patch(MODULE, mock):
        result = query_deconstruct_node({"raw_query": "Where is the PaymentService?"})

    assert "entities" in result
    assert "query_type" in result
    assert "intent" in result
    assert isinstance(result["entities"], list)
    assert len(result["entities"]) > 0


def test_fallback_on_missing_keys():
    mock = MagicMock()
    mock.complete.return_value = json.dumps({"foo": "bar"})
    with patch(MODULE, mock):
        result = query_deconstruct_node({"raw_query": "How does OrderProcessor work?"})

    assert "entities" in result
    assert result["query_type"] in {"WHERE", "HOW", "WHY", "IMPACT"}


def test_fallback_extracts_camel_case():
    mock = MagicMock()
    mock.complete.return_value = "bad json"
    with patch(MODULE, mock):
        result = query_deconstruct_node({"raw_query": "Where is PaymentController defined?"})

    assert any("PaymentController" in e for e in result["entities"])


def test_fallback_does_not_crash_on_empty_query():
    mock = MagicMock()
    mock.complete.return_value = "bad json"
    with patch(MODULE, mock):
        result = query_deconstruct_node({"raw_query": "x"})

    assert "entities" in result
    assert result["entities"]
