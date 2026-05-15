# Project Navigator — CLAUDE.md

## What this is
An AI-native CLI tool that indexes Go and C# repos into a tri-store knowledge graph (Neo4j + Qdrant + PostgreSQL) and answers "Where / How / Why" questions via a LangGraph agent pipeline. Full spec in `PRD.md`.

## Tech stack
- **Language:** Go 1.22
- **Module:** `github.com/navigator/navigator`
- **CLI framework:** Cobra (wired in Task 1.4 — stubbed for now)
- **Stores:** Neo4j 5.18.0 · Qdrant v1.9.2 · PostgreSQL 16.2-alpine

## Directory layout
```
cmd/navigator/main.go       ← binary entry point
internal/cli/               ← Cobra commands          (Epic 1.4)
internal/ingestion/         ← ETL pipeline            (Epic 1.2)
internal/graph/             ← Neo4j client + Cypher   (Epic 1.3)
internal/orchestrator/      ← LangGraph agents        (Epic 1.5)
internal/config/            ← Layered config loader   (Task 1.1.3)
internal/models/            ← Shared domain types
tests/                      ← Integration tests (-tags integration)
scripts/health_check.sh     ← Pre-ingestion health check (exits 2 on failure)
.github/workflows/ci.yml    ← CI: lint + test on push/PR
docker-compose.yml          ← Local infra (neo4j, qdrant, postgres)
.env.example                ← All required env vars documented
```

## Task execution workflow
All tasks come from `PRD.md`. Run them with `/task [NUMBER]` (e.g. `/task 1.2.1`).
The skill reads PRD.md, loads relevant sub-skills from `.claude/commands/`, plans, waits for approval, then implements.

## Skill → task type map
| Task involves… | Load skill |
|---|---|
| LangGraph nodes, agent state, routing | `langgraph` |
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
- CLI-first (not IDE plugin)
- Hybrid LLM: local embeddings + Claude API for reasoning
- Graph + Vector search (not pure vector)
- Go is the sole implementation language
- Default model: `claude-sonnet-4-6`
- No `latest` Docker tags — all images pinned
- No secrets in committed files — `.env` only (gitignored)

## Output conventions
- User-facing content → stdout
- Logs, progress, errors → stderr
- Exit codes: `0` success · `1` user error (bad args) · `2` system error (infra unreachable)
- Integration tests use `-tags integration` build tag; run with `make test-integration`
