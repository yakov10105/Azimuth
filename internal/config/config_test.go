package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// requiredEnv sets the three required env vars and returns a cleanup func.
func requiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("NEO4J_PASSWORD", "testpass")
	t.Setenv("POSTGRES_DSN", "postgresql://azimuth:testpass@localhost:5432/azimuth")
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-test")
}

func TestLoad_Defaults(t *testing.T) {
	requiredEnv(t)

	cfg, err := Load("")
	require.NoError(t, err)

	assert.Equal(t, "bolt://localhost:7687", cfg.Neo4j.URI)
	assert.Equal(t, "neo4j", cfg.Neo4j.User)
	assert.Equal(t, "http://localhost:6333", cfg.Qdrant.URL)
	assert.Equal(t, "claude-sonnet-4-6", cfg.LLM.Model)
	assert.Equal(t, "info", cfg.App.LogLevel)
	assert.Equal(t, 8, cfg.App.WorkerPoolSize)
	assert.Equal(t, 3, cfg.App.GraphWalkerDepth)
	assert.Equal(t, "text-embedding-3-large", cfg.App.EmbeddingModel)
	assert.Equal(t, 100, cfg.App.EmbeddingBatchSize)
	assert.Equal(t, 500, cfg.App.Neo4jWriteBatchSize)
}

func TestLoad_MissingPassword(t *testing.T) {
	t.Setenv("POSTGRES_DSN", "postgresql://azimuth:testpass@localhost:5432/azimuth")
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-test")

	_, err := Load("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "NEO4J_PASSWORD")
}

func TestLoad_MissingDSN(t *testing.T) {
	t.Setenv("NEO4J_PASSWORD", "testpass")
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-test")

	_, err := Load("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "POSTGRES_DSN")
}

func TestLoad_MissingAPIKey(t *testing.T) {
	t.Setenv("NEO4J_PASSWORD", "testpass")
	t.Setenv("POSTGRES_DSN", "postgresql://azimuth:testpass@localhost:5432/azimuth")

	_, err := Load("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ANTHROPIC_API_KEY")
}

func TestLoad_MultipleRequired(t *testing.T) {
	_, err := Load("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "NEO4J_PASSWORD")
	assert.Contains(t, err.Error(), "POSTGRES_DSN")
	assert.Contains(t, err.Error(), "ANTHROPIC_API_KEY")
}

func TestLoad_EnvOverridesYAML(t *testing.T) {
	requiredEnv(t)
	t.Setenv("LOG_LEVEL", "warn")

	// Write a YAML file that sets log_level to debug
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(cfgFile, []byte("app:\n  log_level: debug\n"), 0o644)
	require.NoError(t, err)

	cfg, err := Load(cfgFile)
	require.NoError(t, err)
	assert.Equal(t, "warn", cfg.App.LogLevel)
}

func TestLoad_InvalidNeo4jURI(t *testing.T) {
	requiredEnv(t)
	t.Setenv("NEO4J_URI", "http://localhost:7687")

	_, err := Load("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "NEO4J_URI")
}

func TestLoad_InvalidQdrantURL(t *testing.T) {
	requiredEnv(t)
	t.Setenv("QDRANT_URL", "bolt://localhost:6333")

	_, err := Load("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "QDRANT_URL")
}
