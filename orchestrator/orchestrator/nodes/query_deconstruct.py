import json
import logging
import re

from ..llm.anthropic_client import AnthropicClient
from ..llm.client import LLMClient
from ..state import AzimuthState

logger = logging.getLogger(__name__)

_SYSTEM_PROMPT = """
You are a query analysis agent for Azimuth, a codebase intelligence tool.

Given a developer's natural-language question about a codebase, extract:
- entities: the specific code symbols, components, or concepts mentioned
- intent: a concise one-sentence restatement of what the developer wants to know
- query_type: one of WHERE | HOW | WHY | IMPACT

Definitions:
- WHERE: locating where something is defined or lives in the codebase
- HOW: understanding the mechanics or flow of how something works
- WHY: understanding the historical or design reason behind a decision
- IMPACT: understanding the downstream effects of a potential change

Always respond with valid JSON matching this schema exactly:
{
  "entities": ["<string>", ...],
  "intent": "<string>",
  "query_type": "WHERE" | "HOW" | "WHY" | "IMPACT"
}

Examples:

Query: "Where is the payment handler?"
Response: {"entities": ["payment handler"], "intent": "Locate the payment handler in the codebase", "query_type": "WHERE"}

Query: "How does the retry logic work in the order service?"
Response: {"entities": ["retry logic", "order service"], "intent": "Understand the retry mechanism in the order service", "query_type": "HOW"}

Query: "Why is there a mutex around the user cache?"
Response: {"entities": ["mutex", "user cache"], "intent": "Understand the design rationale for locking the user cache", "query_type": "WHY"}

Query: "If I change the User struct, what will break?"
Response: {"entities": ["User"], "intent": "Identify downstream components affected by changes to the User struct", "query_type": "IMPACT"}

Query: "Where is authentication middleware defined?"
Response: {"entities": ["authentication middleware"], "intent": "Find the definition of the authentication middleware", "query_type": "WHERE"}

Query: "How does the database connection pool get initialised?"
Response: {"entities": ["database connection pool"], "intent": "Understand the initialisation flow for the database connection pool", "query_type": "HOW"}

Respond with JSON only. Do not include any explanation or markdown fencing.
""".strip()

# Module-level singleton — patch this in unit tests
_llm_client: LLMClient = AnthropicClient()

# Patterns for fallback entity extraction
_CAMEL_CASE = re.compile(r"\b[A-Z][a-zA-Z0-9]+(?:[A-Z][a-zA-Z0-9]+)+\b")
_SNAKE_CASE = re.compile(r"\b[a-z][a-z0-9]*(?:_[a-z0-9]+){1,}\b")
_QUOTED = re.compile(r'["\']([^"\']{2,})["\']')
_CAPITALISED = re.compile(r"\b[A-Z][a-z]{2,}\b")


def _fallback(raw_query: str) -> dict:
    """Regex-based entity extraction used when the LLM returns malformed output."""
    entities: list[str] = []
    for pattern in (_QUOTED, _CAMEL_CASE, _SNAKE_CASE, _CAPITALISED):
        entities.extend(pattern.findall(raw_query))
    seen: set[str] = set()
    unique = [e for e in entities if not (e in seen or seen.add(e))]  # type: ignore[func-returns-value]
    if not unique:
        unique = [w for w in raw_query.split() if len(w) > 3][:3]
    logger.warning("query_deconstruct fallback triggered for query=%r", raw_query)
    return {
        "entities": unique or [raw_query],
        "intent": raw_query,
        "query_type": "WHERE",
    }


def query_deconstruct_node(state: AzimuthState) -> dict:
    raw_query = state["raw_query"]
    try:
        raw = _llm_client.complete(prompt=raw_query, system=_SYSTEM_PROMPT)
        parsed = json.loads(raw)
        entities = parsed["entities"]
        intent = parsed["intent"]
        query_type = parsed["query_type"]
        if query_type not in {"WHERE", "HOW", "WHY", "IMPACT"}:
            raise ValueError(f"unexpected query_type: {query_type!r}")
        if not isinstance(entities, list) or not entities:
            raise ValueError("entities must be a non-empty list")
        logger.debug(
            "query_deconstruct query_type=%s entities=%s",
            query_type,
            entities,
        )
        return {"entities": entities, "intent": intent, "query_type": query_type}
    except (json.JSONDecodeError, KeyError, ValueError) as exc:
        logger.warning("query_deconstruct parse error: %s — using fallback", exc)
        return _fallback(raw_query)
