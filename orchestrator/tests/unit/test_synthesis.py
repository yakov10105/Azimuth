import json
from unittest.mock import MagicMock, patch

import pytest

from orchestrator.nodes.synthesis import synthesis_node

MODULE = "orchestrator.nodes.synthesis._llm_client"


def _llm_response(
    summary: str = "The handler is in payments/handler.go",
    call_path: list[str] | None = None,
    relevant_files: list[str] | None = None,
    self_check_issues_found: bool = False,
) -> str:
    return json.dumps({
        "summary": summary,
        "call_path": call_path or ["HandlePayment (payments/handler.go:38)"],
        "relevant_files": relevant_files or ["payments/handler.go"],
        "self_check_issues_found": self_check_issues_found,
    })


def _state(**kwargs) -> dict:
    base = {
        "raw_query": "Where is the payment handler?",
        "retrieved_nodes": [
            {"fqn": "payments.HandlePayment", "name": "HandlePayment",
             "file_path": "payments/handler.go", "kind": "Function", "in_degree": 3}
        ],
        "retrieved_edges": [{"from_fqn": "main.Run", "to_fqn": "payments.HandlePayment", "type": "CALLS"}],
        "code_snippets": [
            {"file": "payments/handler.go", "start_line": 38, "end_line": 55,
             "content": "func HandlePayment(w http.ResponseWriter, r *http.Request) {}"}
        ],
    }
    base.update(kwargs)
    return base


# ---------------------------------------------------------------------------
# Happy path
# ---------------------------------------------------------------------------

def test_returns_summary_and_call_path():
    mock = MagicMock()
    mock.complete.return_value = _llm_response()
    with patch(MODULE, mock):
        result = synthesis_node(_state())
    assert result["final_answer"] == "The handler is in payments/handler.go"
    assert result["call_path"] == ["HandlePayment (payments/handler.go:38)"]
    assert result["relevant_files"] == ["payments/handler.go"]
    assert "error" not in result


def test_no_second_call_when_self_check_clean():
    mock = MagicMock()
    mock.complete.return_value = _llm_response(self_check_issues_found=False)
    with patch(MODULE, mock):
        synthesis_node(_state())
    assert mock.complete.call_count == 1


def test_self_correction_triggered_on_issues():
    mock = MagicMock()
    first  = _llm_response(summary="bad answer", self_check_issues_found=True)
    second = _llm_response(summary="corrected answer", self_check_issues_found=False)
    mock.complete.side_effect = [first, second]
    with patch(MODULE, mock):
        result = synthesis_node(_state())
    assert mock.complete.call_count == 2
    assert result["final_answer"] == "corrected answer"


def test_second_call_prompt_includes_previous_answer():
    mock = MagicMock()
    first  = _llm_response(summary="bad answer", self_check_issues_found=True)
    second = _llm_response(summary="fixed")
    mock.complete.side_effect = [first, second]
    with patch(MODULE, mock):
        synthesis_node(_state())
    correction_prompt = mock.complete.call_args_list[1][1]["prompt"]
    assert "bad answer" in correction_prompt  # previous answer embedded


def test_correction_pass_malformed_falls_back_to_first():
    mock = MagicMock()
    first  = _llm_response(summary="first answer", self_check_issues_found=True)
    mock.complete.side_effect = [first, "not valid json"]
    with patch(MODULE, mock):
        result = synthesis_node(_state())
    # Falls back to first answer when correction is malformed
    assert result["final_answer"] == "first answer"


# ---------------------------------------------------------------------------
# Error / edge cases
# ---------------------------------------------------------------------------

def test_malformed_json_returns_error():
    mock = MagicMock()
    mock.complete.return_value = "this is not json"
    with patch(MODULE, mock):
        result = synthesis_node(_state())
    assert "error" in result
    assert "final_answer" not in result


def test_missing_keys_returns_error():
    mock = MagicMock()
    mock.complete.return_value = json.dumps({"summary": "oops"})  # missing required keys
    with patch(MODULE, mock):
        result = synthesis_node(_state())
    assert "error" in result


def test_empty_context_returns_error():
    mock = MagicMock()
    with patch(MODULE, mock):
        result = synthesis_node(_state(retrieved_nodes=[], code_snippets=[]))
    mock.complete.assert_not_called()
    assert "error" in result


def test_context_includes_file_path_from_snippet():
    mock = MagicMock()
    mock.complete.return_value = _llm_response()
    with patch(MODULE, mock):
        synthesis_node(_state())
    prompt_sent = mock.complete.call_args[1]["prompt"]
    assert "payments/handler.go" in prompt_sent


def test_context_includes_edges():
    mock = MagicMock()
    mock.complete.return_value = _llm_response()
    with patch(MODULE, mock):
        synthesis_node(_state())
    prompt_sent = mock.complete.call_args[1]["prompt"]
    assert "main.Run" in prompt_sent
