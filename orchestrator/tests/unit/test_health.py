import pytest
from fastapi.testclient import TestClient
from unittest.mock import AsyncMock, patch

from orchestrator.main import app

client = TestClient(app)


def test_healthz_returns_ok():
    response = client.get("/healthz")
    assert response.status_code == 200
    assert response.json() == {"status": "ok"}


def test_ask_returns_503_when_neo4j_unreachable():
    from neo4j.exceptions import ServiceUnavailable

    with patch("orchestrator.main.AsyncGraphDatabase.driver") as mock_driver_cls:
        mock_driver = AsyncMock()
        mock_driver.verify_connectivity.side_effect = ServiceUnavailable("connection refused")
        mock_driver.close = AsyncMock()
        mock_driver_cls.return_value = mock_driver

        response = client.post("/ask", json={"question": "Where is the payment handler?"})

    assert response.status_code == 503
    assert "Neo4j" in response.json()["detail"]


def test_ask_stub_returns_not_yet_implemented():
    with patch("orchestrator.main.AsyncGraphDatabase.driver") as mock_driver_cls:
        mock_driver = AsyncMock()
        mock_driver.verify_connectivity = AsyncMock(return_value=None)
        mock_driver.close = AsyncMock()
        mock_driver_cls.return_value = mock_driver

        response = client.post("/ask", json={"question": "Where is the payment handler?"})

    assert response.status_code == 200
    body = response.json()
    assert body["summary"] == "not yet implemented"
    assert body["call_path"] == []
    assert body["relevant_files"] == []


def test_ask_validates_depth_bounds():
    with patch("orchestrator.main.AsyncGraphDatabase.driver") as mock_driver_cls:
        mock_driver = AsyncMock()
        mock_driver.verify_connectivity = AsyncMock(return_value=None)
        mock_driver.close = AsyncMock()
        mock_driver_cls.return_value = mock_driver

        # depth=0 is below minimum of 1
        response = client.post("/ask", json={"question": "test", "depth": 0})
    assert response.status_code == 422

    # depth=11 is above maximum of 10
    response = client.post("/ask", json={"question": "test", "depth": 11})
    assert response.status_code == 422
