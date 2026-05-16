//go:build integration

package graph

import (
	"context"
	"os"
	"testing"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/stretchr/testify/require"
)

// TestSchemaManager_Integration_Migrate connects to a real Neo4j instance and
// runs Migrate twice to verify that all constraints and indices are applied
// without error on the first run and are idempotent on the second.
//
// Requires environment variables:
//
//	NEO4J_URI      — bolt URI (e.g. bolt://localhost:7687)
//	NEO4J_USER     — username (default: neo4j)
//	NEO4J_PASSWORD — password
func TestSchemaManager_Integration_Migrate(t *testing.T) {
	uri := env("NEO4J_URI", "bolt://localhost:7687")
	user := env("NEO4J_USER", "neo4j")
	pass := env("NEO4J_PASSWORD", "")

	if pass == "" {
		t.Skip("NEO4J_PASSWORD not set — skipping integration test")
	}

	driver, err := neo4j.NewDriverWithContext(uri, neo4j.BasicAuth(user, pass, ""))
	require.NoError(t, err, "failed to create Neo4j driver")
	defer driver.Close(context.Background())

	require.NoError(t, driver.VerifyConnectivity(context.Background()),
		"Neo4j unreachable at %s", uri)

	sm := NewSchemaManager()

	// First run — creates constraints and indices.
	require.NoError(t, sm.Migrate(context.Background(), driver),
		"first Migrate call must succeed")

	// Second run — must succeed unchanged (IF NOT EXISTS semantics).
	require.NoError(t, sm.Migrate(context.Background(), driver),
		"second Migrate call must be idempotent")
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
