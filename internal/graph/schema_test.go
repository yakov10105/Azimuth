package graph

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchemaStatements_AllIdempotent(t *testing.T) {
	for _, stmt := range schemaStatements {
		assert.Contains(t, stmt, "IF NOT EXISTS",
			"statement must be idempotent: %q", stmt)
	}
}

func TestSchemaStatements_CoversAllLabels(t *testing.T) {
	joined := strings.Join(schemaStatements, "\n")
	for _, label := range []string{"Function", "Method", "Class", "Struct", "Interface"} {
		assert.Contains(t, joined, label,
			"no constraint or index found for label %s", label)
	}
}

func TestSchemaManager_migrate_ExecutesAllStatements(t *testing.T) {
	sm := NewSchemaManager()

	var executed []string
	runner := func(_ context.Context, stmt string) error {
		executed = append(executed, stmt)
		return nil
	}

	require.NoError(t, sm.migrate(context.Background(), runner))
	assert.Len(t, executed, len(schemaStatements),
		"every schema statement must be executed exactly once")
}

func TestSchemaManager_migrate_StopsOnError(t *testing.T) {
	sm := NewSchemaManager()

	callCount := 0
	runner := func(_ context.Context, _ string) error {
		callCount++
		if callCount == 2 {
			return errors.New("neo4j: connection refused")
		}
		return nil
	}

	err := sm.migrate(context.Background(), runner)
	require.Error(t, err)
	assert.Equal(t, 2, callCount, "should stop executing after the first error")
	assert.Contains(t, err.Error(), "neo4j: connection refused")
}
