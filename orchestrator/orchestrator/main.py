import logging
from contextlib import asynccontextmanager

from fastapi import FastAPI, HTTPException

from .agent import graph
from .config import LOG_LEVEL
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
    result = await graph.ainvoke({
        "raw_query": request.question,
        "depth": request.depth,
        # Initialise accumulated list fields so operator.add never receives None
        "retrieved_nodes": [],
        "retrieved_edges": [],
        "code_snippets": [],
        "historical_context": [],
    })

    if result.get("error"):
        logger.error("graph pipeline error: %s", result["error"])
        raise HTTPException(status_code=500, detail=result["error"])

    return AskResponse(
        summary=result.get("final_answer") or "",
        call_path=result.get("call_path") or [],
        relevant_files=result.get("relevant_files") or [],
    )
