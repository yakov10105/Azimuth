import logging

from neo4j import AsyncGraphDatabase
from neo4j.exceptions import ServiceUnavailable  # noqa: F401 — re-exported for callers

from .queries import FIND_BY_NAME, FIND_ENTRY_POINTS, GET_SUBGRAPH

logger = logging.getLogger(__name__)


class Neo4jReadClient:
    """Read-only Neo4j client. One instance per process; driver manages the pool."""

    def __init__(self, uri: str, user: str, password: str) -> None:
        self._driver = AsyncGraphDatabase.driver(
            uri,
            auth=(user, password),
            max_connection_pool_size=50,
            connection_timeout=5.0,
        )

    async def find_candidates(self, name: str) -> list[dict]:
        """Fuzzy name search — returns up to 10 nodes whose name contains `name`."""
        async with self._driver.session() as session:
            result = await session.run(FIND_BY_NAME, {"name": name})
            records = await result.data()
            logger.debug("find_candidates name=%r returned %d records", name, len(records))
            return records

    async def find_entry_points(self, fqns: list[str], limit: int = 5) -> list[dict]:
        """Re-rank a set of known FQNs by in-degree; return top `limit`."""
        if not fqns:
            return []
        async with self._driver.session() as session:
            result = await session.run(FIND_ENTRY_POINTS, {"fqns": fqns, "limit": limit})
            records = await result.data()
            logger.debug("find_entry_points fqns=%d returned %d records", len(fqns), len(records))
            return records

    async def get_subgraph(self, fqn: str, depth: int) -> tuple[list[dict], list[dict]]:
        """N-hop CALLS/IMPLEMENTS subgraph from root FQN; returns (nodes, edges)."""
        async with self._driver.session() as session:
            result = await session.run(GET_SUBGRAPH, {"fqn": fqn, "depth": depth})
            record = await result.single()
            if not record:
                logger.debug("get_subgraph fqn=%r not found", fqn)
                return [], []
            nodes = record["nodes"] or []
            edges = record["edges"] or []
            logger.debug(
                "get_subgraph fqn=%r depth=%d nodes=%d edges=%d",
                fqn, depth, len(nodes), len(edges),
            )
            return nodes, edges

    async def close(self) -> None:
        await self._driver.close()
