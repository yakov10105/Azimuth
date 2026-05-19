import os

from langgraph.graph import END, StateGraph

from .config import LANGCHAIN_API_KEY, LANGCHAIN_PROJECT
from .nodes.entry_point_finder import entry_point_finder_node
from .nodes.graph_walker import graph_walker_node
from .nodes.query_deconstruct import query_deconstruct_node
from .nodes.synthesis import synthesis_node
from .state import AzimuthState

# Re-export under the name the PRD specifies
AgentState = AzimuthState

# Enable LangSmith tracing when an API key is present — no crash when absent
if LANGCHAIN_API_KEY:
    os.environ.setdefault("LANGCHAIN_TRACING_V2", "true")
    os.environ.setdefault("LANGCHAIN_API_KEY", LANGCHAIN_API_KEY)
    os.environ.setdefault("LANGCHAIN_PROJECT", LANGCHAIN_PROJECT)


def build_graph():
    g = StateGraph(AzimuthState)

    g.add_node("deconstruct", query_deconstruct_node)
    g.add_node("find_entry",  entry_point_finder_node)
    g.add_node("walk_graph",  graph_walker_node)
    g.add_node("synthesize",  synthesis_node)

    g.set_entry_point("deconstruct")
    g.add_edge("deconstruct", "find_entry")
    g.add_edge("find_entry",  "walk_graph")
    g.add_edge("walk_graph",  "synthesize")
    g.add_edge("synthesize",  END)

    return g.compile()


# Compiled once at import time — reused across all requests
graph = build_graph()
