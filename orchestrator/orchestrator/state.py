import operator
from typing import Annotated, Optional

from typing_extensions import TypedDict


class AzimuthState(TypedDict):
    # Inputs — set once at graph entry, never mutated
    raw_query: str
    depth: Optional[int]  # graph traversal depth; defaults to GRAPH_WALKER_DEPTH if None

    # Node 1 output
    query_type: Optional[str]   # "WHERE" | "HOW" | "WHY" | "IMPACT"
    intent: Optional[str]
    entities: Optional[list[str]]

    # Node 2 output
    entry_points: Optional[list[dict]]

    # Node 3 output — accumulated across multiple traversal steps
    retrieved_nodes: Annotated[list[dict], operator.add]
    retrieved_edges: Annotated[list[dict], operator.add]
    code_snippets: Annotated[list[dict], operator.add]

    # Node 4 (historian) output — accumulated
    historical_context: Annotated[list[dict], operator.add]

    # Final output — set by synthesis node
    final_answer: Optional[str]
    call_path: Optional[list[str]]
    relevant_files: Optional[list[str]]
    diagram_mermaid: Optional[str]

    # Error routing — any node may set this; error_handler_node consumes it
    error: Optional[str]
