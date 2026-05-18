//go:build integration

package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupIngestTest connects to Neo4j and returns a driver.
// Skips the test if NEO4J_PASSWORD is unset.
func setupIngestTest(t *testing.T) neo4j.DriverWithContext {
	t.Helper()
	uri := envOr("NEO4J_URI", "bolt://localhost:7687")
	user := envOr("NEO4J_USER", "neo4j")
	pass := os.Getenv("NEO4J_PASSWORD")
	if pass == "" {
		t.Skip("NEO4J_PASSWORD not set — skipping integration test")
	}
	driver, err := neo4j.NewDriverWithContext(uri, neo4j.BasicAuth(user, pass, ""))
	require.NoError(t, err)
	require.NoError(t, driver.VerifyConnectivity(context.Background()),
		"Neo4j unreachable at %s", uri)
	t.Cleanup(func() { driver.Close(context.Background()) })
	return driver
}

// countNodesWithPrefix returns the count of nodes (any label) whose fqn or
// path starts with the given prefix. Used to verify ingest wrote expected nodes.
func countNodesWithPrefix(t *testing.T, driver neo4j.DriverWithContext, prefix string) int64 {
	t.Helper()
	session := driver.NewSession(context.Background(), neo4j.SessionConfig{})
	defer session.Close(context.Background())
	result, err := session.Run(context.Background(),
		`MATCH (n) WHERE (n.fqn STARTS WITH $p OR n.path STARTS WITH $p) RETURN count(n) AS cnt`,
		map[string]any{"p": prefix},
	)
	require.NoError(t, err)
	record, err := result.Single(context.Background())
	require.NoError(t, err)
	return record.Values[0].(int64)
}

// deleteNodesWithPrefix removes test nodes written during the integration run.
func deleteNodesWithPrefix(t *testing.T, driver neo4j.DriverWithContext, prefix string) {
	t.Helper()
	session := driver.NewSession(context.Background(), neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(context.Background())
	result, err := session.Run(context.Background(),
		`MATCH (n) WHERE (n.fqn STARTS WITH $p OR n.path STARTS WITH $p) DETACH DELETE n`,
		map[string]any{"p": prefix},
	)
	require.NoError(t, err)
	_, err = result.Consume(context.Background())
	require.NoError(t, err)
}

// makeGitRepo creates a temp directory, copies src files into it, and runs
// git init so validateIngestArgs accepts it as a repository.
func makeGitRepo(t *testing.T, srcFiles map[string]string) string {
	t.Helper()
	dir := t.TempDir()

	err := exec.Command("git", "-C", dir, "init").Run()
	require.NoError(t, err, "git init failed")

	for name, content := range srcFiles {
		dest := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(dest, []byte(content), 0644))
	}
	return dir
}

// envOr returns the value of an env var or a default.
func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// TestIngest_Integration_E2E ingests a minimal fixture repo and verifies that
// Neo4j contains Function nodes for the two Go functions in the fixture.
func TestIngest_Integration_E2E(t *testing.T) {
	driver := setupIngestTest(t)

	// Build a fixture repo with two known functions.
	repoDir := makeGitRepo(t, map[string]string{
		"main.go": `package main

import "fmt"

func Greet(name string) { fmt.Println(Hello(name)) }
func Hello(name string) string { return "Hello, " + name }
`,
	})

	// Use the repo path as a unique prefix for cleanup.
	prefix := repoDir
	t.Cleanup(func() { deleteNodesWithPrefix(t, driver, prefix) })

	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetOut(outBuf)
	cmd.SetErr(errBuf)

	opts := IngestOptions{
		RepoPath: repoDir,
		Language: "go",
	}

	err := runIngest(context.Background(), cmd, opts)
	require.NoError(t, err, "runIngest failed: stdout=%s stderr=%s", outBuf, errBuf)

	// Summary must be on stdout.
	assert.Contains(t, outBuf.String(), "Ingestion complete")
	assert.Contains(t, outBuf.String(), "Files:")
	assert.Contains(t, outBuf.String(), "Nodes:")

	// Neo4j must have at least the File node and the two Function nodes.
	nodeCount := countNodesWithPrefix(t, driver, prefix)
	assert.GreaterOrEqual(t, nodeCount, int64(3),
		"expected ≥3 nodes (1 File + 2 Functions), got %d", nodeCount)
}

// TestIngest_Integration_DryRun verifies that --dry-run parses but writes nothing.
func TestIngest_Integration_DryRun(t *testing.T) {
	driver := setupIngestTest(t)

	repoDir := makeGitRepo(t, map[string]string{
		"main.go": `package drytest
func OnlyForDryRun() {}
`,
	})

	prefix := repoDir

	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetOut(outBuf)
	cmd.SetErr(errBuf)

	opts := IngestOptions{RepoPath: repoDir, Language: "go", DryRun: true}
	err := runIngest(context.Background(), cmd, opts)
	require.NoError(t, err)

	assert.Contains(t, outBuf.String(), "dry-run")
	assert.Contains(t, outBuf.String(), "would write")

	// Dry run must not write any nodes.
	nodeCount := countNodesWithPrefix(t, driver, prefix)
	assert.Equal(t, int64(0), nodeCount,
		fmt.Sprintf("dry-run must not write nodes, found %d", nodeCount))
}
