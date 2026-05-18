import logging

from neo4j.exceptions import ServiceUnavailable

from ..config import NEO4J_PASSWORD, NEO4J_URI, NEO4J_USER
from ..graph.client import Neo4jReadClient
from ..state import AzimuthState

logger = logging.getLogger(__name__)

# Module-level singleton — replace in unit tests
_neo4j_client: Neo4jReadClient = Neo4jReadClient(NEO4J_URI, NEO4J_USER, NEO4J_PASSWORD)


async def entry_point_finder_node(state: AzimuthState) -> dict:
    entities: list[str] = state.get("entities") or []

    if not entities:
        logger.warning("entry_point_finder received empty entities list")
        return {"entry_points": [], "error": "No entities provided by query deconstruction"}

    try:
        # Phase 1: fuzzy-match each entity — collect all candidate records
        seen: dict[str, dict] = {}
        for entity in entities:
            candidates = await _neo4j_client.find_candidates(entity)
            for record in candidates:
                fqn = record["fqn"]
                # Keep the record with the highest in_degree when the same FQN appears
                # for multiple entity terms
                if fqn not in seen or record["in_degree"] > seen[fqn]["in_degree"]:
                    seen[fqn] = record

        if not seen:
            logger.info("entry_point_finder no candidates for entities=%s", entities)
            return {
                "entry_points": [],
                "error": f"No nodes found for entities: {entities}",
            }

        # Phase 2: re-rank deduplicated candidates by in-degree and return top 5
        top5 = await _neo4j_client.find_entry_points(fqns=list(seen), limit=5)

        if not top5:
            return {
                "entry_points": [],
                "error": f"No entry points found for entities: {entities}",
            }

        logger.debug("entry_point_finder returning %d entry points", len(top5))
        return {"entry_points": top5}

    except ServiceUnavailable as exc:
        logger.error("entry_point_finder Neo4j unreachable: %s", exc)
        return {"error": f"Neo4j unreachable: {exc}"}
