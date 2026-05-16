package graph

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// schemaStatements are all Cypher DDL statements that define the graph schema.
// Every statement uses IF NOT EXISTS so the slice is safe to re-execute on every startup.
var schemaStatements = []string{
	// ── Uniqueness constraints ──────────────────────────────────────────────
	// Constraints also create a backing index automatically.
	`CREATE CONSTRAINT file_path IF NOT EXISTS FOR (f:File) REQUIRE f.path IS UNIQUE`,
	`CREATE CONSTRAINT function_fqn IF NOT EXISTS FOR (f:Function) REQUIRE f.fqn IS UNIQUE`,
	`CREATE CONSTRAINT method_fqn IF NOT EXISTS FOR (m:Method) REQUIRE m.fqn IS UNIQUE`,
	`CREATE CONSTRAINT class_fqn IF NOT EXISTS FOR (c:Class) REQUIRE c.fqn IS UNIQUE`,
	`CREATE CONSTRAINT struct_fqn IF NOT EXISTS FOR (s:Struct) REQUIRE s.fqn IS UNIQUE`,
	`CREATE CONSTRAINT interface_fqn IF NOT EXISTS FOR (i:Interface) REQUIRE i.fqn IS UNIQUE`,

	// ── Additional lookup indices ───────────────────────────────────────────
	// fqn is already indexed by the constraints above.
	// These indices support fuzzy name searches used by the query library.
	`CREATE INDEX function_name_idx IF NOT EXISTS FOR (f:Function) ON (f.name)`,
	`CREATE INDEX class_name_idx IF NOT EXISTS FOR (c:Class) ON (c.name)`,
}

// SchemaManager applies Neo4j schema constraints and indices.
type SchemaManager struct{}

// NewSchemaManager creates a new SchemaManager.
func NewSchemaManager() *SchemaManager { return &SchemaManager{} }

// Migrate applies all schema constraints and indices to Neo4j.
// It is idempotent: safe to call on every application startup.
func (sm *SchemaManager) Migrate(ctx context.Context, driver neo4j.DriverWithContext) error {
	session := driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	return sm.migrate(ctx, func(ctx context.Context, stmt string) error {
		result, err := session.Run(ctx, stmt, nil)
		if err != nil {
			return err
		}
		_, err = result.Consume(ctx)
		return err
	})
}

// migrate executes all schema statements through the provided runner.
// Unexported so tests can inject a capturing runner without a real driver.
func (sm *SchemaManager) migrate(ctx context.Context, runner func(context.Context, string) error) error {
	for _, stmt := range schemaStatements {
		if err := runner(ctx, stmt); err != nil {
			return fmt.Errorf("schema migrate %q: %w", abbreviated(stmt), err)
		}
	}
	return nil
}

// abbreviated truncates s to 60 characters for use in error messages.
func abbreviated(s string) string {
	const maxLen = 60
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
