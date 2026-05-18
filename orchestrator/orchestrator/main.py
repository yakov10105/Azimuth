import logging
import logging.config
from contextlib import asynccontextmanager

from fastapi import FastAPI, HTTPException
from neo4j import AsyncGraphDatabase
from neo4j.exceptions import ServiceUnavailable

from .config import LOG_LEVEL, NEO4J_PASSWORD, NEO4J_URI, NEO4J_USER
from .schemas import AskRequest, AskResponse

logging.basicConfig(level=LOG_LEVEL)
logger = logging.getLogger(__name__)


@asynccontextmanager
async def lifespan(app: FastAPI):
    logger.info("Azimuth orchestrator starting")
    yield
    logger.info("Azimuth orchestrator stopped")


app = FastAPI(title="Azimuth Orchestrator", version="0.1.0", lifespan=lifespan)


@app.get("/healthz")
async def healthz() -> dict:
    return {"status": "ok"}


@app.post("/ask", response_model=AskResponse)
async def ask(request: AskRequest) -> AskResponse:
    driver = AsyncGraphDatabase.driver(NEO4J_URI, auth=(NEO4J_USER, NEO4J_PASSWORD))
    try:
        await driver.verify_connectivity()
    except ServiceUnavailable as exc:
        logger.error("Neo4j unreachable at %s: %s", NEO4J_URI, exc)
        raise HTTPException(status_code=503, detail="Neo4j service unavailable") from exc
    finally:
        await driver.close()

    # LangGraph pipeline wired in Task 1.5.6
    return AskResponse(
        summary="not yet implemented",
        call_path=[],
        relevant_files=[],
    )
