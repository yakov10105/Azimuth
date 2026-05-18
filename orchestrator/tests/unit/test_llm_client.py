import logging
from unittest.mock import MagicMock, call, patch

import anthropic
import pytest

from orchestrator.llm.anthropic_client import AnthropicClient, _MAX_ATTEMPTS


def _make_response(text: str = "hello", input_tokens: int = 10, output_tokens: int = 5) -> MagicMock:
    response = MagicMock()
    response.content = [MagicMock(text=text)]
    response.usage.input_tokens = input_tokens
    response.usage.output_tokens = output_tokens
    return response


@patch("orchestrator.llm.anthropic_client.anthropic.Anthropic")
def test_complete_returns_text(MockAnthropic):
    MockAnthropic.return_value.messages.create.return_value = _make_response("world")
    client = AnthropicClient(api_key="sk-test")
    assert client.complete("q", "sys") == "world"


@patch("orchestrator.llm.anthropic_client.anthropic.Anthropic")
def test_complete_logs_token_usage(MockAnthropic, caplog):
    MockAnthropic.return_value.messages.create.return_value = _make_response(
        input_tokens=42, output_tokens=7
    )
    with caplog.at_level(logging.DEBUG, logger="orchestrator.llm.anthropic_client"):
        client = AnthropicClient(api_key="sk-test")
        client.complete("q", "sys")
    assert "input_tokens=42" in caplog.text
    assert "output_tokens=7" in caplog.text


@patch("orchestrator.llm.anthropic_client.time.sleep")
@patch("orchestrator.llm.anthropic_client.anthropic.Anthropic")
def test_retry_on_rate_limit_succeeds(MockAnthropic, mock_sleep):
    create = MockAnthropic.return_value.messages.create
    create.side_effect = [
        anthropic.RateLimitError("rate limited", response=MagicMock(), body={}),
        anthropic.RateLimitError("rate limited", response=MagicMock(), body={}),
        _make_response("ok"),
    ]
    client = AnthropicClient(api_key="sk-test")
    assert client.complete("q", "sys") == "ok"
    assert create.call_count == 3
    assert mock_sleep.call_count == 2
    mock_sleep.assert_has_calls([call(1), call(2)])


@patch("orchestrator.llm.anthropic_client.time.sleep")
@patch("orchestrator.llm.anthropic_client.anthropic.Anthropic")
def test_retry_on_internal_server_error_succeeds(MockAnthropic, mock_sleep):
    create = MockAnthropic.return_value.messages.create
    create.side_effect = [
        anthropic.InternalServerError("server error", response=MagicMock(), body={}),
        _make_response("ok"),
    ]
    client = AnthropicClient(api_key="sk-test")
    assert client.complete("q", "sys") == "ok"
    assert create.call_count == 2
    mock_sleep.assert_called_once_with(1)


@patch("orchestrator.llm.anthropic_client.time.sleep")
@patch("orchestrator.llm.anthropic_client.anthropic.Anthropic")
def test_exhausted_retries_raises(MockAnthropic, mock_sleep):
    err = anthropic.RateLimitError("rate limited", response=MagicMock(), body={})
    MockAnthropic.return_value.messages.create.side_effect = err
    client = AnthropicClient(api_key="sk-test")
    with pytest.raises(anthropic.RateLimitError):
        client.complete("q", "sys")
    assert MockAnthropic.return_value.messages.create.call_count == _MAX_ATTEMPTS


@patch("orchestrator.llm.anthropic_client.anthropic.Anthropic")
def test_api_key_not_in_logs(MockAnthropic, caplog):
    secret_key = "sk-ant-super-secret-12345"
    MockAnthropic.return_value.messages.create.return_value = _make_response()
    with caplog.at_level(logging.DEBUG, logger="orchestrator.llm.anthropic_client"):
        client = AnthropicClient(api_key=secret_key)
        client.complete("q", "sys")
    assert secret_key not in caplog.text
