# Azimuth — CLAUDE.md

## What this is
An AI-native CLI tool (`zm`) that indexes Go and C# repos into a tri-store knowledge graph (Neo4j + Qdrant + PostgreSQL) and answers "Where / How / Why" questions via a LangGraph agent pipeline. Full spec in `PRD.md`.

## Tech stack
- **Languages:** Go 1.23 (ingestion + CLI) · Python 3.12 (orchestrator service)
- **Go module:** `github.com/azimuth/azimuth`
- **CLI framework:** Cobra
- **CLI command:** `zm`
- **Orchestrator:** Python · LangGraph 0.2 · FastAPI · Anthropic Python SDK
- **Stores:** Neo4j 5.18.0 · Qdrant v1.9.2 · PostgreSQL 16.2-alpine

## Directory layout
```
cmd/zm/main.go              ← Go binary entry point
internal/cli/               ← Cobra commands          (Epic 1.4)
internal/ingestion/         ← ETL pipeline            (Epic 1.2)
internal/graph/             ← Neo4j client + Cypher   (Epic 1.3)
internal/config/            ← Layered config loader   (Task 1.1.3 ✅)
internal/models/            ← Shared domain types
orchestrator/               ← Python LangGraph service (Epic 1.5)
  orchestrator/main.py      ← FastAPI entry point (POST /ask)
  orchestrator/agent.py     ← LangGraph StateGraph definition
  orchestrator/nodes/       ← Agent nodes 1–4
  orchestrator/llm/         ← LLM client (Anthropic SDK)
  orchestrator/graph/       ← Neo4j read-only client + Cypher queries
  orchestrator/schemas.py   ← Pydantic request/response models
  pyproject.toml            ← Python package manifest
  Dockerfile                ← Orchestrator container image
tests/                      ← Go integration tests (-tags integration)
scripts/health_check.sh     ← Pre-ingestion health check (exits 2 on failure)
.github/workflows/ci.yml    ← CI: secret scan + lint + test on push/PR
docker-compose.yml          ← Local infra (neo4j, qdrant, postgres, orchestrator)
.env.example                ← All required env vars documented
config.example.yaml         ← Example YAML config
```

## Task execution workflow
All tasks come from `PRD.md`. Run them with `/task [NUMBER]` (e.g. `/task 1.2.1`).
The skill reads PRD.md, loads relevant sub-skills from `.claude/commands/`, plans, waits for approval, then implements.

## Skill → task type map
| Task involves… | Load skill |
|---|---|
| LangGraph nodes, agent state, routing (Python) | `langgraph` |
| Anthropic SDK, Claude API, LLM client | `claude-api` |
| Qdrant, embeddings, vector search | `rag` |
| Go code, goroutines, go test, modules | `go-dev` |
| C#, .NET, ASP.NET, Roslyn | `csharp-dev` |
| Neo4j, Cypher, graph schema | `neo4j` |
| PostgreSQL, migrations, SQL | `postgres` |
| Tree-sitter, AST parsing | `tree-sitter` |
| CLI subcommands, flags, stdout/stderr | `cli-dev` |
| Docker Compose, health checks, CI | `infra` |
| Git log, git blame, subprocess | `git-ingestion` |

## Locked architectural decisions
- CLI-first (not IDE plugin); binary name is `zm`
- Hybrid LLM: local embeddings + Claude API for reasoning
- Graph + Vector search (not pure vector)
- **CQRS split:** Go owns the write path (ingestion → Neo4j/Qdrant/PostgreSQL); Python LangGraph owns the read path (query → answer). Neither service calls the other at runtime — they communicate only through shared stores.
- Go for ingestion + CLI; Python 3.12 + LangGraph for the orchestrator service
- `zm ask` is a thin Go HTTP client; the Python service runs on `ORCHESTRATOR_URL` (default `http://localhost:8000`)
- Default LLM model: `claude-sonnet-4-6`
- No `latest` Docker tags — all images pinned
- No secrets in committed files — `.env` only (gitignored)

## Output conventions
- User-facing content → stdout
- Logs, progress, errors → stderr
- Exit codes: `0` success · `1` user error (bad args) · `2` system error (infra unreachable)
- Go integration tests use `-tags integration` build tag; run with `make test-integration`
- Python tests run with `pytest`; run with `make orchestrator-test`
- Neo4j schema in `/docs/graph-schema.md` is the shared contract between Go (writer) and Python (reader)
