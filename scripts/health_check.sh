#!/usr/bin/env bash
# Run before ingestion to confirm all three stores are reachable. Exits 2 on any failure.
set -euo pipefail

fail() { echo "FAIL: $1" >&2; exit 2; }
ok()   { echo "OK:   $1"; }

# Neo4j HTTP
curl -sf "http://localhost:7474" > /dev/null && ok "Neo4j HTTP" || fail "Neo4j not reachable at http://localhost:7474"

# Qdrant
curl -sf "http://localhost:6333/healthz" > /dev/null && ok "Qdrant" || fail "Qdrant not reachable at http://localhost:6333"

# PostgreSQL
pg_isready -h localhost -U "${POSTGRES_USER:-navigator}" && ok "PostgreSQL" || fail "PostgreSQL not reachable at localhost:5432"

echo ""
echo "All systems operational."
