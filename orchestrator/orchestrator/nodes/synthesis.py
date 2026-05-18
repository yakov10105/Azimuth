import json
import logging

from ..llm.anthropic_client import AnthropicClient
from ..llm.client import LLMClient
from ..state import AzimuthState

logger = logging.getLogger(__name__)

_SYSTEM_PROMPT = """
You are the Synthesis agent for Azimuth, a codebase intelligence tool.

You are given:
- A developer's question about a codebase
- Code nodes from the knowledge graph (functions, methods, classes) with file paths
- Call graph edges showing how those nodes relate
- Actual source code snippets with exact file paths and line numbers

Your task is to answer the developer's question precisely and concisely.

Rules:
- Cite ONLY file paths that appear in the provided code snippets — never invent paths
- Format every entry in call_path as: "FunctionName (path/to/file.go:lineNumber)"
- relevant_files must be a subset of the files in the provided snippets
- Keep summary under 300 words

After writing your answer, perform a self-check:
- Does your answer cite a file path NOT in the provided snippets? → self_check_issues_found: true
- Does your answer describe behaviour that contradicts the provided code? → self_check_issues_found: true
- Otherwise → self_check_issues_found: false

Output format — valid JSON only, no markdown fencing:
{
  "summary": "<concise answer to the question>",
  "call_path": ["EntryPoint (file.go:10)", "Callee (other.go:55)"],
  "relevant_files": ["file.go", "other.go"],
  "self_check_issues_found": false
}

Examples:

Question: Where is the payment handler?
Snippets: payments/handler.go (lines 38-55) — func HandlePayment(w http.ResponseWriter, r *http.Request) { ... }
Nodes: Function `payments.HandlePayment` in payments/handler.go
Response:
{"summary": "The payment handler is `HandlePayment` defined in `payments/handler.go` at line 38. It accepts an HTTP request and processes the payment flow.", "call_path": ["HandlePayment (payments/handler.go:38)"], "relevant_files": ["payments/handler.go"], "self_check_issues_found": false}

Question: How does the retry logic work in the order service?
Snippets: orders/retry.go (lines 12-40) — func RetryWithBackoff(op func() error, maxAttempts int) error { ... }
Nodes: Function `orders.RetryWithBackoff` in orders/retry.go
Response:
{"summary": "The retry logic is implemented in `RetryWithBackoff` (orders/retry.go:12). It accepts an operation function and a max-attempt count, sleeping with exponential backoff between each attempt.", "call_path": ["RetryWithBackoff (orders/retry.go:12)"], "relevant_files": ["orders/retry.go"], "self_check_issues_found": false}

Question: Why is there a mutex around the user cache?
Snippets: cache/users.go (lines 5-20) — var mu sync.RWMutex // guards concurrent reads/writes added after race detected in production
Nodes: Struct `cache.UserCache` in cache/users.go
Response:
{"summary": "The mutex in `cache/users.go` was introduced to guard concurrent reads and writes to the user cache after a data race was detected in production (see comment at line 6).", "call_path": [], "relevant_files": ["cache/users.go"], "self_check_issues_found": false}
""".strip()

_CORRECTION_PREFIX = (
    "Your previous answer had self-check issues. "
    "Review the code snippets carefully and produce a corrected answer "
    "using only information present in the provided context.\n\n"
    "Previous answer:\n"
)

# Module-level singleton — replace in unit tests
_llm_client: LLMClient = AnthropicClient()


def _build_context(nodes: list[dict], edges: list[dict], snippets: list[dict]) -> str:
    parts: list[str] = []

    if snippets:
        parts.append("## Code Snippets\n")
        for s in snippets:
            parts.append(
                f"### {s['file']} (lines {s['start_line']}–{s['end_line']})\n"
                f"```\n{s['content']}\n```\n"
            )

    if nodes:
        parts.append("## Knowledge Graph Nodes\n")
        for n in nodes:
            parts.append(
                f"- {n.get('kind', 'Node')} `{n.get('fqn')}` "
                f"in `{n.get('file_path')}` "
                f"(in-degree: {n.get('in_degree', 0)})\n"
            )

    if edges:
        parts.append("## Call Graph Edges\n")
        for e in edges:
            parts.append(f"- `{e.get('from_fqn')}` → `{e.get('to_fqn')}` [{e.get('type', 'CALLS')}]\n")

    return "\n".join(parts) if parts else "(no context available)"


def _build_user_prompt(query: str, context: str) -> str:
    return f"Question: {query}\n\n{context}"


def _parse_synthesis(raw: str) -> dict | None:
    try:
        parsed = json.loads(raw)
        _ = parsed["summary"]
        _ = parsed["call_path"]
        _ = parsed["relevant_files"]
        _ = parsed["self_check_issues_found"]
        return parsed
    except (json.JSONDecodeError, KeyError, TypeError):
        return None


def synthesis_node(state: AzimuthState) -> dict:
    raw_query = state.get("raw_query", "")
    nodes: list[dict] = state.get("retrieved_nodes") or []
    edges: list[dict] = state.get("retrieved_edges") or []
    snippets: list[dict] = state.get("code_snippets") or []

    if not nodes and not snippets:
        logger.warning("synthesis_node called with no context")
        return {"error": "No context available to synthesise an answer"}

    context = _build_context(nodes, edges, snippets)
    user_prompt = _build_user_prompt(raw_query, context)

    raw = _llm_client.complete(prompt=user_prompt, system=_SYSTEM_PROMPT)
    parsed = _parse_synthesis(raw)

    if parsed is None:
        logger.warning("synthesis_node first call produced malformed JSON")
        return {"error": "Synthesis produced malformed output — LLM response was not valid JSON"}

    logger.debug(
        "synthesis_node first_call self_check_issues_found=%s",
        parsed.get("self_check_issues_found"),
    )

    # Self-correction pass — triggered at most once
    if parsed.get("self_check_issues_found"):
        logger.info("synthesis_node triggering correction pass")
        correction_prompt = _CORRECTION_PREFIX + json.dumps(parsed) + "\n\n" + user_prompt
        raw2 = _llm_client.complete(prompt=correction_prompt, system=_SYSTEM_PROMPT)
        parsed2 = _parse_synthesis(raw2)
        if parsed2 is not None:
            parsed = parsed2
            logger.debug("synthesis_node correction pass accepted")
        else:
            logger.warning("synthesis_node correction pass also malformed — using first answer")

    return {
        "final_answer": parsed["summary"],
        "call_path": parsed["call_path"],
        "relevant_files": parsed["relevant_files"],
    }
