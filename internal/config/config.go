// Package config implements layered configuration loading:
// built-in defaults → config.yaml → environment variables.
package config

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all runtime configuration for Azimuth.
type Config struct {
	Neo4j    Neo4jConfig
	Qdrant   QdrantConfig
	Postgres PostgresConfig
	LLM      LLMConfig
	App      AppConfig
}

// Neo4jConfig holds connection details for the graph store.
type Neo4jConfig struct {
	URI      string
	User     string
	Password string
}

// QdrantConfig holds connection details for the vector store.
type QdrantConfig struct {
	URL string
}

// PostgresConfig holds connection details for the metadata store.
type PostgresConfig struct {
	DSN string
}

// LLMConfig holds LLM provider settings.
type LLMConfig struct {
	AnthropicAPIKey string
	Model           string
}

// AppConfig holds application-level tuning parameters.
type AppConfig struct {
	LogLevel            string
	TargetRepoPath      string
	WorkerPoolSize      int
	GraphWalkerDepth    int
	EmbeddingModel      string
	EmbeddingBatchSize  int
	Neo4jWriteBatchSize int
}

// Load reads configuration in order of increasing precedence:
// built-in defaults → cfgFile (or config.yaml) → environment variables.
// Returns a validated Config or an error listing all violations.
func Load(cfgFile string) (*Config, error) {
	v := viper.New()

	setDefaults(v)

	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
	}

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("config: read file: %w", err)
		}
	}

	bindEnvVars(v)

	cfg := unmarshal(v)

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks required fields and URI/URL formats.
// Returns a single error listing all violations so the operator can fix them in one pass.
func (c *Config) Validate() error {
	var errs []string

	if c.Neo4j.Password == "" {
		errs = append(errs, "NEO4J_PASSWORD is required")
	}
	if c.Postgres.DSN == "" {
		errs = append(errs, "POSTGRES_DSN is required")
	}
	if c.LLM.AnthropicAPIKey == "" {
		errs = append(errs, "ANTHROPIC_API_KEY is required")
	}

	if c.Neo4j.URI != "" && !strings.HasPrefix(c.Neo4j.URI, "bolt://") {
		errs = append(errs, fmt.Sprintf("NEO4J_URI must be a bolt:// URI, got: %q", c.Neo4j.URI))
	}

	if c.Qdrant.URL != "" {
		u, err := url.Parse(c.Qdrant.URL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			errs = append(errs, fmt.Sprintf("QDRANT_URL must be an http(s):// URL, got: %q", c.Qdrant.URL))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("configuration errors:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("neo4j.uri", "bolt://localhost:7687")
	v.SetDefault("neo4j.user", "neo4j")
	v.SetDefault("qdrant.url", "http://localhost:6333")
	v.SetDefault("llm.model", "claude-sonnet-4-6")
	v.SetDefault("app.log_level", "info")
	v.SetDefault("app.worker_pool_size", 8)
	v.SetDefault("app.graph_walker_depth", 3)
	v.SetDefault("app.embedding_model", "text-embedding-3-large")
	v.SetDefault("app.embedding_batch_size", 100)
	v.SetDefault("app.neo4j_write_batch_size", 500)
}

func bindEnvVars(v *viper.Viper) {
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	pairs := [][2]string{
		{"neo4j.uri", "NEO4J_URI"},
		{"neo4j.user", "NEO4J_USER"},
		{"neo4j.password", "NEO4J_PASSWORD"},
		{"qdrant.url", "QDRANT_URL"},
		{"postgres.dsn", "POSTGRES_DSN"},
		{"llm.anthropic_api_key", "ANTHROPIC_API_KEY"},
		{"llm.model", "LLM_MODEL"},
		{"app.log_level", "LOG_LEVEL"},
		{"app.target_repo_path", "TARGET_REPO_PATH"},
		{"app.worker_pool_size", "WORKER_POOL_SIZE"},
		{"app.graph_walker_depth", "GRAPH_WALKER_DEPTH"},
		{"app.embedding_model", "EMBEDDING_MODEL"},
		{"app.embedding_batch_size", "EMBEDDING_BATCH_SIZE"},
		{"app.neo4j_write_batch_size", "NEO4J_WRITE_BATCH_SIZE"},
	}
	for _, p := range pairs {
		_ = v.BindEnv(p[0], p[1])
	}
}

func unmarshal(v *viper.Viper) *Config {
	return &Config{
		Neo4j: Neo4jConfig{
			URI:      v.GetString("neo4j.uri"),
			User:     v.GetString("neo4j.user"),
			Password: v.GetString("neo4j.password"),
		},
		Qdrant: QdrantConfig{
			URL: v.GetString("qdrant.url"),
		},
		Postgres: PostgresConfig{
			DSN: v.GetString("postgres.dsn"),
		},
		LLM: LLMConfig{
			AnthropicAPIKey: v.GetString("llm.anthropic_api_key"),
			Model:           v.GetString("llm.model"),
		},
		App: AppConfig{
			LogLevel:            v.GetString("app.log_level"),
			TargetRepoPath:      v.GetString("app.target_repo_path"),
			WorkerPoolSize:      v.GetInt("app.worker_pool_size"),
			GraphWalkerDepth:    v.GetInt("app.graph_walker_depth"),
			EmbeddingModel:      v.GetString("app.embedding_model"),
			EmbeddingBatchSize:  v.GetInt("app.embedding_batch_size"),
			Neo4jWriteBatchSize: v.GetInt("app.neo4j_write_batch_size"),
		},
	}
}
