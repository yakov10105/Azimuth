# Project Navigator

An AI-native developer tool that treats your codebase as a Living Knowledge Graph — answering "How", "Where", and "Why" questions using AST structure, call graphs, and Git history.

## Prerequisites

- Go 1.22+
- Docker + Docker Compose
- `golangci-lint` v1.59+

## Quick Start

```bash
cp .env.example .env
# edit .env — set NEO4J_PASSWORD, POSTGRES_PASSWORD, ANTHROPIC_API_KEY

make infra-up
make build
./bin/navigator --help
```

## Make Targets

| Target | Description |
|---|---|
| `make build` | Compile the navigator binary to `./bin/navigator` |
| `make test` | Run unit tests |
| `make test-integration` | Run integration tests (requires running infra) |
| `make lint` | Run golangci-lint |
| `make run` | Run the CLI directly via `go run` |
| `make infra-up` | Start Neo4j, Qdrant, and PostgreSQL via Docker Compose |
| `make infra-down` | Stop all services |
| `make infra-reset` | Destroy volumes and restart all services |
| `make health` | Check that all services are reachable |
| `make db-migrate` | Apply pending database migrations |
| `make benchmark` | Run benchmarks |
